package seeders

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/auth/hash"
	"github.com/AlexGalhardo/vukem/config"
	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/factory"
	"github.com/AlexGalhardo/vukem/orm"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/catalog"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/models"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/payments"
)

// DemoSellerDomain is the email domain of every demo catalog seller; it
// identifies them for idempotency and DemoCatalog.Purge.
const DemoSellerDomain = "vendedores.example.com"

// DemoCatalogSeed is the default fake-data seed: the same seed always
// produces the same sellers, shops and products.
const DemoCatalogSeed uint64 = 20261018

// Demo catalog defaults: 100 sellers with 10–30 products each, exactly 20
// on average (2000 products).
const (
	DefaultDemoSellers     = 100
	DefaultDemoMinProducts = 10
	DefaultDemoMaxProducts = 30
)

// ErrDemoCatalogInProduction stops the demo catalog in production unless
// it was asked for explicitly (demo:catalog --allow-production).
var ErrDemoCatalogInProduction = errors.New(
	"seeders: the demo catalog is not seeded in production without demo:catalog --allow-production")

// sellerChunk sellers (with their shops, products and images) are written
// per transaction: a seller is either fully seeded or absent.
const sellerChunk = 25

// DemoCatalog seeds demo sellers, one shop each, and their published
// products with shared placeholder images, then demo buyers who bought
// from those shops over the last six months (orders, reviews with votes
// and seller replies) and asked them questions. It is deterministic (Seed)
// and idempotent: sellers and buyers whose email exists are skipped, and
// so is the activity of buyers who already have some. Sellers and buyers
// get random passwords nobody knows; they are not meant to log in.
type DemoCatalog struct {
	Sellers     int    // default DefaultDemoSellers
	MinProducts int    // default DefaultDemoMinProducts
	MaxProducts int    // default DefaultDemoMaxProducts
	Seed        uint64 // default DemoCatalogSeed
	// Buyers is the number of demo buyers (default DefaultDemoBuyers);
	// negative seeds none.
	Buyers int
	// MaxOrders: each buyer places 1..MaxOrders orders (default
	// DefaultDemoMaxOrders).
	MaxOrders int
	// MaxQuestions: each buyer asks 0..MaxQuestions questions (default
	// DefaultDemoMaxQuestions).
	MaxQuestions int
	// FeeBPS is the platform fee of the simulated orders; zero uses
	// PLATFORM_FEE_BPS (RunApp) or payments.DefaultFeeBPS.
	FeeBPS int64
	// AllowProduction is the explicit opt-in for APP_ENV=production.
	AllowProduction bool
	// Env overrides the app's environment (tests); empty uses the app's.
	Env config.Env
	// Hasher hashes the random passwords; nil uses hash.Default().
	Hasher hash.Hasher
}

// DemoCatalogResult reports what one run created.
type DemoCatalogResult struct {
	Sellers   int
	Products  int
	Buyers    int
	Orders    int
	Reviews   int
	Questions int
	Elapsed   time.Duration
}

// Name implements the db:seed name hook.
func (DemoCatalog) Name() string { return "DemoCatalog" }

// SeedApp implements vukem.AppSeeder.
func (d DemoCatalog) SeedApp(ctx context.Context, app *vukem.App) error {
	res, err := d.RunApp(ctx, app)
	if err != nil {
		return err
	}

	app.Log.InfoContext(ctx, "demo catalog seeded",
		"sellers", res.Sellers, "products", res.Products, "buyers", res.Buyers, "orders", res.Orders,
		"reviews", res.Reviews, "questions", res.Questions, "elapsed", res.Elapsed.Round(time.Millisecond))

	return nil
}

// RunApp checks the environment, then runs the seeder on the app's
// database and image disk.
func (d DemoCatalog) RunApp(ctx context.Context, app *vukem.App) (DemoCatalogResult, error) {
	if d.Env == "" {
		d.Env = app.Config.Env
	}

	if err := d.allowed(); err != nil {
		return DemoCatalogResult{}, err
	}

	if d.FeeBPS == 0 {
		if raw := strings.TrimSpace(app.Source.Get("PLATFORM_FEE_BPS")); raw != "" {
			bps, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || payments.ValidateFeeBPS(bps) != nil {
				return DemoCatalogResult{}, fmt.Errorf("seed demo catalog: %w", payments.ErrInvalidFeeBPS)
			}

			d.FeeBPS = bps
		}
	}

	db, err := app.DB()
	if err != nil {
		return DemoCatalogResult{}, err
	}

	return d.Run(ctx, db)
}

func (d DemoCatalog) allowed() error {
	if d.Env == config.Production && !d.AllowProduction {
		return ErrDemoCatalogInProduction
	}

	return nil
}

func (d DemoCatalog) withDefaults() DemoCatalog {
	if d.Sellers <= 0 {
		d.Sellers = DefaultDemoSellers
	}

	if d.MinProducts <= 0 {
		d.MinProducts = DefaultDemoMinProducts
	}

	if d.MaxProducts < d.MinProducts {
		d.MaxProducts = max(DefaultDemoMaxProducts, d.MinProducts)
	}

	if d.Seed == 0 {
		d.Seed = DemoCatalogSeed
	}

	if d.Buyers == 0 {
		d.Buyers = DefaultDemoBuyers
	}

	if d.MaxOrders <= 0 {
		d.MaxOrders = DefaultDemoMaxOrders
	}

	if d.MaxQuestions <= 0 {
		d.MaxQuestions = DefaultDemoMaxQuestions
	}

	if d.FeeBPS <= 0 {
		d.FeeBPS = payments.DefaultFeeBPS
	}

	if d.Hasher == nil {
		d.Hasher = hash.Default()
	}

	return d
}

// Run seeds the catalog into db; its images are embedded in the binary.
// It does not check the environment: SeedApp and demo:catalog do.
func (d DemoCatalog) Run(ctx context.Context, db *database.DB) (DemoCatalogResult, error) {
	start := time.Now()
	d = d.withDefaults()

	if err := (Categories{}).Seed(ctx, db); err != nil {
		return DemoCatalogResult{}, err
	}

	catIDs, err := categoryIDs(ctx, db)
	if err != nil {
		return DemoCatalogResult{}, err
	}

	images := sharedImages()

	now := time.Now().UTC()
	plan := d.plan(catIDs, images, now)

	todo, err := missingSellers(ctx, db, plan)
	if err != nil {
		return DemoCatalogResult{}, err
	}

	users := make([]*models.User, len(todo))
	for i := range todo {
		users[i] = &todo[i].user
	}

	if err := d.hashPasswords(users); err != nil {
		return DemoCatalogResult{}, err
	}

	res := DemoCatalogResult{}

	for chunk := range slicesChunk(todo, sellerChunk) {
		if err := database.RunInTx(
			ctx,
			db,
			func(tx *database.Tx) error { return insertSellers(ctx, tx, chunk) },
		); err != nil {
			return DemoCatalogResult{}, fmt.Errorf("seed demo catalog: %w", err)
		}

		for i := range chunk {
			res.Sellers++
			res.Products += len(chunk[i].products)
		}
	}

	if d.Buyers > 0 {
		act, err := d.seedActivity(ctx, db, now)
		if err != nil {
			return DemoCatalogResult{}, err
		}

		res.Buyers, res.Orders, res.Reviews, res.Questions = act.Buyers, act.Orders, act.Reviews, act.Questions
	}

	res.Elapsed = time.Since(start)

	return res, nil
}

// ErrDemoPurgeBlocked stops DemoCatalog.Purge (demo:catalog --fresh) when
// rows of real users hang off the demo catalog: deleting the demo rows
// would delete or change them. Nothing is deleted then.
var ErrDemoPurgeBlocked = errors.New("seeders: real data references the demo catalog; nothing was deleted")

// Subqueries selecting the demo users (sellers and buyers, by their email
// domain), the demo shops and the demo products. Each "?" pair takes
// demoPatterns().
//
// A demo shop that a real buyer ordered from is kept with its owner and
// products (keptShopsSQL): deleting it would delete the real order. It is
// left out of every demo set below, so Purge removes the rest of the demo
// catalog around it.
const (
	baseUsersSQL = `SELECT id FROM users WHERE email LIKE ? OR email LIKE ?`
	baseShopsSQL = `SELECT id FROM shops WHERE owner_id IN (` + baseUsersSQL + `)`

	keptShopsSQL = `SELECT shop_id FROM seller_orders WHERE shop_id IN (` + baseShopsSQL +
		`) AND order_id NOT IN (SELECT id FROM orders WHERE buyer_id IN (` + baseUsersSQL + `))`
	keptOwnersSQL = `SELECT owner_id FROM shops WHERE id IN (` + keptShopsSQL + `)`

	demoUsersSQL    = `SELECT id FROM users WHERE (email LIKE ? OR email LIKE ?) AND id NOT IN (` + keptOwnersSQL + `)`
	demoShopsSQL    = `SELECT id FROM shops WHERE owner_id IN (` + demoUsersSQL + `)`
	demoProductsSQL = `SELECT id FROM products WHERE shop_id IN (` + demoShopsSQL + `)`
	demoOrdersSQL   = `SELECT id FROM orders WHERE buyer_id IN (` + demoUsersSQL + `)`
	demoReviewsSQL  = `SELECT id FROM reviews WHERE product_id IN (` + demoProductsSQL + `)`

	// Reviews by demo buyers of the kept shops' products go with the demo buyers.
	keptReviewsByDemoSQL = `SELECT id FROM reviews WHERE buyer_id IN (` + demoUsersSQL +
		`) AND product_id IN (SELECT id FROM products WHERE shop_id IN (` + keptShopsSQL + `))`
)

func demoPatterns() []any { return []any{"%@" + DemoSellerDomain, "%@" + DemoBuyerDomain} }

// purgeBlockers are the real-user rows Purge refuses to touch, each a
// COUNT query whose placeholders are all demoPatterns() pairs.
var purgeBlockers = []struct {
	what, sql string
}{
	{"sales of real shops to demo buyers", `SELECT COUNT(*) FROM seller_orders WHERE order_id IN (` + demoOrdersSQL +
		`) AND shop_id NOT IN (` + demoShopsSQL + `) AND shop_id NOT IN (` + keptShopsSQL + `)`},
	{"ledger entries of demo shops or orders", `SELECT COUNT(*) FROM ledger_entries WHERE shop_id IN (` +
		demoShopsSQL + `) OR order_id IN (` + demoOrdersSQL + `)`},
	{"payouts of demo shops", `SELECT COUNT(*) FROM payouts WHERE shop_id IN (` + demoShopsSQL + `)`},
	{"reviews by real users of demo products", `SELECT COUNT(*) FROM reviews WHERE product_id IN (` +
		demoProductsSQL + `) AND buyer_id NOT IN (` + demoUsersSQL + `)`},
	{"real users' votes on demo reviews", `SELECT COUNT(*) FROM review_votes WHERE review_id IN (` +
		demoReviewsSQL + `) AND user_id NOT IN (` + demoUsersSQL + `)`},
	{"demo users' votes on real reviews", `SELECT COUNT(*) FROM review_votes WHERE user_id IN (` + demoUsersSQL +
		`) AND review_id NOT IN (` + demoReviewsSQL + `) AND review_id NOT IN (` + keptReviewsByDemoSQL + `)`},
	{"questions by real users on demo products", `SELECT COUNT(*) FROM product_questions WHERE shop_id IN (` +
		demoShopsSQL + `) AND asker_id NOT IN (` + demoUsersSQL + `)`},
	{"real users' cart items with demo products", `SELECT COUNT(*) FROM cart_items WHERE product_id IN (` +
		demoProductsSQL + `) AND user_id NOT IN (` + demoUsersSQL + `)`},
}

// Purge deletes the demo catalog: the demo buyers' orders (with their
// seller orders, lines, reviews and votes), the demo shops (products,
// images, questions) and the demo users (addresses, questions, votes), in
// that order. It never deletes or changes a real user's row: demo shops with
// a real order are kept whole, and when a real row references the rest of the
// demo catalog (a real review,
// question, vote or cart item, a ledger entry or payout) it fails with
// ErrDemoPurgeBlocked, listing them, and deletes nothing.
func (DemoCatalog) Purge(ctx context.Context, db *database.DB) error {
	return database.RunInTx(ctx, db, func(tx *database.Tx) error {
		d := tx.Dialect()

		var blocked []string

		for _, b := range purgeBlockers {
			var n int64

			args := slices.Repeat(demoPatterns(), strings.Count(b.sql, "?")/2)
			if err := tx.QueryRowContext(ctx, database.Rebind(d, b.sql), args...).Scan(&n); err != nil {
				return fmt.Errorf("purge demo catalog: count %s: %w", b.what, err)
			}

			if n > 0 {
				blocked = append(blocked, fmt.Sprintf("%d %s", n, b.what))
			}
		}

		if len(blocked) > 0 {
			return fmt.Errorf("%w: %s", ErrDemoPurgeBlocked, strings.Join(blocked, ", "))
		}

		for _, step := range []struct{ what, sql string }{
			{"orders", `DELETE FROM orders WHERE id IN (` + demoOrdersSQL + `)`},
			{"shops", `DELETE FROM shops WHERE id IN (` + demoShopsSQL + `)`},
			{"users", `DELETE FROM users WHERE id IN (` + demoUsersSQL + `)`},
		} {
			args := slices.Repeat(demoPatterns(), strings.Count(step.sql, "?")/2)
			if _, err := tx.ExecContext(ctx, database.Rebind(d, step.sql), args...); err != nil {
				return fmt.Errorf("purge demo %s: %w", step.what, err)
			}
		}

		return nil
	})
}

// slicesChunk yields consecutive chunks of s of at most n items.
func slicesChunk[T any](s []T, n int) func(yield func([]T) bool) {
	return func(yield func([]T) bool) {
		for len(s) > 0 {
			k := min(n, len(s))
			if !yield(s[:k]) {
				return
			}

			s = s[k:]
		}
	}
}

func categoryIDs(ctx context.Context, db *database.DB) (map[string]int64, error) {
	cats, err := orm.QueryOf[catalog.Category](db).Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("load categories: %w", err)
	}

	out := make(map[string]int64, len(cats))
	for i := range cats {
		out[cats[i].Slug] = cats[i].ID
	}

	return out, nil
}

// plannedSeller is one seller and everything it owns, ids not yet set.
type plannedSeller struct {
	user     models.User
	shop     catalog.Shop
	products []plannedProduct
}

type plannedProduct struct {
	product catalog.Product
	images  []catalog.ProductImage
}

// productCounts returns n counts in [lo, hi] averaging (lo+hi)/2: each
// random count is paired with its mirror (lo+hi-c).
func productCounts(f *factory.Faker, n, lo, hi int) []int {
	out := make([]int, n)

	for i := 0; i+1 < n; i += 2 {
		c := f.Int(lo, hi)
		out[i], out[i+1] = c, lo+hi-c
	}

	if n%2 == 1 {
		out[n-1] = (lo + hi) / 2
	}

	return out
}

// plan builds the whole catalog in memory. It depends only on the seed
// (and now, for published_at), so a re-run plans the same sellers.
func (d DemoCatalog) plan(
	catIDs map[string]int64,
	images map[string][]catalog.ProductImage,
	now time.Time,
) []plannedSeller {
	counts := productCounts(factory.NewFaker(d.Seed), d.Sellers, d.MinProducts, d.MaxProducts)

	users := factory.New(func(f *factory.Faker) models.User {
		first, last := f.FirstName(), f.LastName()
		local := strings.ReplaceAll(catalog.Slugify(first+" "+last), "-", ".")

		return models.User{
			Name:  first + " " + last,
			Email: fmt.Sprintf("%s.%03d@%s", local, f.Seq(), DemoSellerDomain),
			Role:  models.RoleUser,
		}
	}).Seed(d.Seed).Count(d.Sellers).State(func(u *models.User) {
		u.EmailVerifiedAt.V, u.EmailVerifiedAt.Valid = now, true
	}).Make()

	out := make([]plannedSeller, d.Sellers)

	for i := range out {
		f := factory.NewFaker(d.Seed + uint64(i) + 1)
		main := demoCategories[i%len(demoCategories)]
		second := demoCategories[(i*5+3)%len(demoCategories)]
		shop := demoShop(f, i, users[i], main.slug)

		products := factory.New(func(f *factory.Faker) plannedProduct {
			cat := main
			if f.Int(1, 10) > 7 {
				cat = second
			}

			return demoProduct(f, cat, catIDs[cat.slug], images[cat.slug], shop.ShipFromUF,
				fmt.Sprintf("%03d%02d", i+1, f.Seq()), now)
		}).Seed(d.Seed ^ uint64(i+1)<<20).Count(counts[i]).Make()

		out[i] = plannedSeller{user: users[i], shop: shop, products: products}
	}

	return out
}

func demoShop(f *factory.Faker, i int, owner models.User, mainCategory string) catalog.Shop {
	last := owner.Name[strings.LastIndex(owner.Name, " ")+1:]

	var name string
	if f.Bool() {
		name = factory.Pick(f, shopPrefixes...) + " " + last
	} else {
		name = last + " " + factory.Pick(f, shopSuffixes[mainCategory]...)
	}

	desc := fmt.Sprintf("%s %s Loja de demonstração: dados fictícios gerados para desenvolvimento.",
		factory.Pick(f, shopSlogans...), factory.Pick(f, shopSlogans...))

	return catalog.Shop{
		Slug:        fmt.Sprintf("%s-%03d", catalog.Slugify(name), i+1),
		Name:        name,
		Description: nullStr(desc),
		Status:      catalog.ShopActive,
		ShipFromUF:  catalog.UFs[(i*7)%len(catalog.UFs)], // 7 and 27 are coprime: every UF gets shops
	}
}

func nullStr(s string) sql.Null[string] { return sql.Null[string]{V: s, Valid: true} }

func demoProduct(f *factory.Faker, cat demoCategory, categoryID int64, images []catalog.ProductImage,
	uf, suffix string, now time.Time,
) plannedProduct {
	k := factory.Pick(f, cat.kinds...)
	brand := factory.Pick(f, cat.brands...)

	specs := make([]string, len(k.specs))
	for j, group := range k.specs {
		specs[j] = factory.Pick(f, group...)
	}

	var title string
	if k.model {
		model := factory.Pick(f, "A", "K", "M", "N", "S", "X", "Z") + strconv.Itoa(f.Int(1, 99))
		if series := factory.Pick(f, modelSeries...); series != "" {
			model += " " + series
		}

		title = strings.Join(append([]string{k.noun, brand, model}, specs...), " ")
	} else {
		title = strings.Join(append([]string{k.noun}, specs...), " ") + " - " + brand
	}

	reais := f.Int(k.minReais, k.maxReais)
	cents := int64(reais)*100 - int64(factory.Pick(f, 10, 1, 100)) // R$ x,90 / x,99 / (x-1),00

	var b strings.Builder

	b.WriteString(title + ".\n\nHighlights:\n")

	for _, feat := range k.features {
		b.WriteString("- " + feat + "\n")
	}

	fmt.Fprintf(&b, "\n%s %s Ships from %s.\n\n", factory.Pick(f, productClosings...),
		factory.Pick(f, productClosings...), uf)
	b.WriteString("Anúncio de demonstração: produto e loja fictícios, gerados para desenvolvimento.")

	p := catalog.Product{
		CategoryID:  sql.Null[int64]{V: categoryID, Valid: categoryID != 0},
		Slug:        catalog.Slugify(title) + "-dm" + suffix,
		Title:       title,
		Description: b.String(),
		PriceCents:  max(cents, 100),
		Stock:       f.Int(1, 150),
		WeightGrams: f.Int(k.minGrams, k.maxGrams),
		Status:      catalog.StatusPublished,
	}
	p.PublishedAt.V, p.PublishedAt.Valid = now.Add(-time.Duration(f.Int(0, 180*24*60))*time.Minute), true

	first := f.Int(0, len(images)-1)
	imgs := make([]catalog.ProductImage, f.Int(1, 2))

	for j := range imgs {
		imgs[j] = images[(first+j)%len(images)]
		imgs[j].Position = j
		imgs[j].Alt = title
	}

	return plannedProduct{product: p, images: imgs}
}

// missingSellers keeps the planned sellers whose email is not stored yet.
func missingSellers(ctx context.Context, db *database.DB, plan []plannedSeller) ([]plannedSeller, error) {
	emails := make([]any, len(plan))
	for i := range plan {
		emails[i] = plan[i].user.Email
	}

	have, err := orm.QueryOf[models.User](db).WhereIn("email", emails...).Pluck[string](ctx, "email")
	if err != nil {
		return nil, fmt.Errorf("load demo sellers: %w", err)
	}

	seen := make(map[string]bool, len(have))
	for _, e := range have {
		seen[e] = true
	}

	out := make([]plannedSeller, 0, len(plan))

	for i := range plan {
		if !seen[plan[i].user.Email] {
			out = append(out, plan[i])
		}
	}

	return out, nil
}

// hashPasswords gives each user a random 192-bit password that is never
// stored or printed, hashed on one goroutine per CPU (argon2id is the slow
// part).
func (d DemoCatalog) hashPasswords(users []*models.User) error {
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)

	work := make(chan int)

	for range runtime.NumCPU() {
		wg.Go(func() {
			for i := range work {
				h, err := d.Hasher.Make(rand.Text())
				if err != nil {
					mu.Lock()
					errs = append(errs, err)
					mu.Unlock()

					continue
				}

				users[i].Password = h
			}
		})
	}

	for i := range users {
		work <- i
	}

	close(work)
	wg.Wait()

	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("hash demo passwords: %w", err)
	}

	return nil
}

// insertSellers writes a chunk with one multi-row INSERT per table.
func insertSellers(ctx context.Context, tx *database.Tx, chunk []plannedSeller) error {
	users := make([]models.User, len(chunk))
	for i := range chunk {
		users[i] = chunk[i].user
	}

	if err := orm.CreateMany(ctx, tx, users); err != nil {
		return fmt.Errorf("insert sellers: %w", err)
	}

	shops := make([]catalog.Shop, len(chunk))
	for i := range chunk {
		shops[i] = chunk[i].shop
		shops[i].OwnerID = users[i].ID
	}

	if err := orm.CreateMany(ctx, tx, shops); err != nil {
		return fmt.Errorf("insert shops: %w", err)
	}

	var (
		products []catalog.Product
		owners   []*plannedProduct
	)

	for i := range chunk {
		for j := range chunk[i].products {
			p := &chunk[i].products[j]
			pr := p.product
			pr.ShopID = shops[i].ID
			products = append(products, pr)
			owners = append(owners, p)
		}
	}

	if err := orm.CreateMany(ctx, tx, products); err != nil {
		return fmt.Errorf("insert products: %w", err)
	}

	var images []catalog.ProductImage

	for i, p := range owners {
		for _, img := range p.images {
			img.ProductID = products[i].ID
			images = append(images, img)
		}
	}

	if err := orm.CreateMany(ctx, tx, images); err != nil {
		return fmt.Errorf("insert product images: %w", err)
	}

	return nil
}

// sharedImages returns, per category, the image rows pointing at the
// WebP embedded in the binary (catalog.SharedImage serves them), so the
// seed writes nothing to disk and the images survive any redeploy.
func sharedImages() map[string][]catalog.ProductImage {
	out := make(map[string][]catalog.ProductImage, len(demoCategories))

	for _, cat := range demoCategories {
		out[cat.slug] = []catalog.ProductImage{{
			Key: catalog.SharedKey(cat.slug), Width: catalog.SharedImageSize, Height: catalog.SharedImageSize,
		}}
	}

	return out
}
