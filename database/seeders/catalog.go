package seeders

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"time"

	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/auth/hash"
	"github.com/AlexGalhardo/vukem/config"
	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/orm"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/catalog"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/models"
)

// Categories seeds the top-level categories (EN/PT names). It upserts by
// slug, so it is safe to run again.
type Categories struct{}

var categories = []struct{ slug, en, pt string }{
	{"electronics", "Electronics, Audio & Video", "Eletrônicos, Áudio e Vídeo"},
	{"phones", "Cell Phones", "Celulares e Telefones"},
	{"computers", "Computers", "Informática"},
	{"home", "Home & Furniture", "Casa, Móveis e Decoração"},
	{"appliances", "Appliances", "Eletrodomésticos"},
	{"fashion", "Clothing, Shoes & Bags", "Calçados, Roupas e Bolsas"},
	{"sports", "Sports & Fitness", "Esportes e Fitness"}, //nolint:misspell // Portuguese
	{"beauty", "Beauty & Personal Care", "Beleza e Cuidado Pessoal"},
	{"toys", "Toys & Hobbies", "Brinquedos e Hobbies"},
	{"tools", "Tools", "Ferramentas"},
	{"books", "Books & Comics", "Livros, Revistas e Comics"},
	{"car-parts", "Car Parts & Accessories", "Acessórios para Veículos"},
}

// Seed implements vukem.Seeder.
func (Categories) Seed(ctx context.Context, db *database.DB) error {
	for i, c := range categories {
		existing, err := orm.QueryOf[catalog.Category](db).Where("slug", c.slug).First(ctx)

		switch {
		case errors.Is(err, orm.ErrNotFound):
			cat := catalog.Category{Slug: c.slug, NameEN: c.en, NamePT: c.pt, Position: i}
			if err := orm.Create(ctx, db, &cat); err != nil {
				return fmt.Errorf("seed category %s: %w", c.slug, err)
			}
		case err != nil:
			return fmt.Errorf("seed category %s: %w", c.slug, err)
		default:
			if _, err := orm.QueryOf[catalog.Category](db).Where("id", existing.ID).
				Update(ctx, orm.Set{"name_en": c.en, "name_pt": c.pt, "position": i}); err != nil {
				return fmt.Errorf("seed category %s: %w", c.slug, err)
			}
		}
	}

	return nil
}

// DemoSellerEmail owns the demo shop (the auth stream seeds it too; both
// seeders find-or-create by email).
const DemoSellerEmail = "seller@example.com"

// DemoShop seeds a verified seller, its shop and a dozen published
// products with generated placeholder images (no downloads). It skips
// products that already exist, so it is safe to run again.
//
// workaround(vukem): seeders receive only *database.DB — no app config or
// services — so the image disk is opened from the environment here.
type DemoShop struct {
	// Env overrides the app's environment (tests); empty uses the app's.
	Env config.Env
}

var demoProducts = []struct {
	title, category string
	price           int64
	stock, weight   int
	from, to        color.RGBA
}{
	{"Smartphone Galaxy Nova 128 GB", "phones", 189900, 25, 400, rgb(30, 64, 175), rgb(96, 165, 250)},
	{"Pulse Bluetooth Headphones", "electronics", 15990, 80, 250, rgb(17, 24, 39), rgb(107, 114, 128)},
	{"Ultra Laptop 14\" 16 GB RAM", "computers", 459900, 8, 1800, rgb(55, 65, 81), rgb(156, 163, 175)},
	{"Stainless Steel Coffee Maker 1.2 L", "appliances", 24990, 30, 2100, rgb(127, 29, 29), rgb(248, 113, 113)},
	{"Lightweight Running Shoes", "sports", 29990, 40, 700, rgb(6, 95, 70), rgb(52, 211, 153)},
	{"Organic Cotton T-Shirt", "fashion", 7990, 120, 200, rgb(146, 64, 14), rgb(251, 191, 36)},
	{"Vitamin C Skincare Kit", "beauty", 11990, 50, 450, rgb(157, 23, 77), rgb(244, 114, 182)},
	{"500-Piece Building Blocks", "toys", 13990, 35, 1200, rgb(161, 98, 7), rgb(250, 204, 21)},
	{"Impact Drill 650 W", "tools", 27990, 15, 2300, rgb(30, 58, 138), rgb(251, 146, 60)},
	{"LED Desk Lamp", "home", 8990, 60, 900, rgb(68, 64, 60), rgb(253, 230, 138)},
	{"Book: Practical Go", "books", 6990, 100, 500, rgb(8, 47, 73), rgb(56, 189, 248)},
	{"Magnetic Car Phone Mount", "car-parts", 4990, 200, 150, rgb(63, 63, 70), rgb(161, 161, 170)},
}

func rgb(r, g, b uint8) color.RGBA { return color.RGBA{R: r, G: g, B: b, A: 255} }

// SeedApp implements vukem.AppSeeder: it uses the app's media disk (the
// memory disk in tests).
func (d DemoShop) SeedApp(ctx context.Context, app *vukem.App) error {
	if d.Env == "" {
		d.Env = app.Config.Env
	}

	// The demo seller has a known password: never in production, even with
	// db:seed --force (fail closed like Users).
	if d.Env == config.Production {
		return ErrDemoUsersInProduction
	}

	db, err := app.DB()
	if err != nil {
		return err
	}

	if err := (Categories{}).Seed(ctx, db); err != nil {
		return err
	}

	src := app.Source

	disk, err := catalog.OpenDisk(src, app.Config.Env, app.Kind == vukem.KindTest)
	if err != nil {
		return fmt.Errorf("seed demo shop: %w", err)
	}

	svc, err := catalog.New(catalog.Config{DB: db, Disk: disk})
	if err != nil {
		return err
	}

	seller, err := demoSeller(ctx, db, src.Get("SEED_PASSWORD"))
	if err != nil {
		return err
	}

	if _, err := svc.MyShop(ctx, seller.ID); errors.Is(err, catalog.ErrNoShop) {
		_, err = svc.OpenShop(ctx, seller.ID, catalog.ShopInput{
			Name:        "Vukem Demo Shop",
			Description: "Demo shop seeded for local development: everything here is fake.",
			ShipFromUF:  "SP",
		})
		if err != nil {
			return fmt.Errorf("seed demo shop: %w", err)
		}
	} else if err != nil {
		return err
	}

	for i, p := range demoProducts {
		if err := seedProduct(
			ctx,
			db,
			svc,
			seller.ID,
			i,
			p.title,
			p.category,
			p.price,
			p.stock,
			p.weight,
			p.from,
			p.to,
		); err != nil {
			return err
		}
	}

	return nil
}

// demoSeller finds or creates the verified demo seller.
func demoSeller(ctx context.Context, db *database.DB, password string) (models.User, error) {
	u, err := orm.QueryOf[models.User](db).Where("email", DemoSellerEmail).First(ctx)
	if err == nil {
		if !u.EmailVerified() {
			_, err = orm.QueryOf[models.User](
				db,
			).Where("id", u.ID).
				Update(ctx, orm.Set{"email_verified_at": time.Now().UTC()})
		}

		return u, err
	}

	if !errors.Is(err, orm.ErrNotFound) {
		return models.User{}, fmt.Errorf("seed demo seller: %w", err)
	}

	if password == "" {
		password = DefaultSeedPassword // dev-only demo account, documented in the README
	}

	h, err := hash.Default().Make(password)
	if err != nil {
		return models.User{}, err
	}

	u = models.User{Name: "Demo Seller", Email: DemoSellerEmail, Password: h, Role: models.RoleUser}
	u.EmailVerifiedAt.V, u.EmailVerifiedAt.Valid = time.Now().UTC(), true

	if err := orm.Create(ctx, db, &u); err != nil {
		return models.User{}, fmt.Errorf("seed demo seller: %w", err)
	}

	return u, nil
}

func seedProduct(ctx context.Context, db *database.DB, svc *catalog.Service, sellerID int64, i int,
	title, category string, price int64, stock, weight int, from, to color.RGBA,
) error {
	exists, err := orm.QueryOf[catalog.Product](db).Where("slug", catalog.Slugify(title)).Exists(ctx)
	if err != nil || exists {
		return err
	}

	p, err := svc.CreateProduct(ctx, sellerID, catalog.ProductInput{
		Title:        title,
		Description:  title + ". Demo product: placeholder description and image, not for sale.",
		CategorySlug: category,
		PriceCents:   price,
		Stock:        stock,
		WeightGrams:  weight,
	})
	if err != nil {
		return fmt.Errorf("seed product %q: %w", title, err)
	}

	img, err := placeholder(placeholderSize, from, to, i)
	if err != nil {
		return err
	}

	if _, err := svc.SaveProductImage(
		ctx,
		sellerID,
		p.ID,
		catalog.Slugify(title)+".png",
		bytes.NewReader(img),
	); err != nil {
		return fmt.Errorf("seed image for %q: %w", title, err)
	}

	if _, err := svc.SetProductStatus(ctx, sellerID, p.ID, catalog.StatusPublished); err != nil {
		return fmt.Errorf("publish %q: %w", title, err)
	}

	return nil
}

// placeholderSize is the side of the generated placeholder images.
const placeholderSize = 900

// placeholder draws a size×size PNG: a diagonal gradient with a lighter
// disc whose position varies with i (the layout is the same at any size).
func placeholder(size int, from, to color.RGBA, i int) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	scale := func(v int) int { return v * size / placeholderSize }
	cx, cy, r := scale(300+(i%3)*150), scale(300+(i%4)*100), scale(180+(i%5)*20)

	for y := range size {
		for x := range size {
			t := float64(x+y) / float64(2*size)
			c := color.RGBA{
				R: lerp(from.R, to.R, t), G: lerp(from.G, to.G, t), B: lerp(from.B, to.B, t), A: 255,
			}

			if dx, dy := x-cx, y-cy; dx*dx+dy*dy < r*r {
				c.R, c.G, c.B = c.R/2+128, c.G/2+128, c.B/2+128
			}

			img.SetRGBA(x, y, c)
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode placeholder: %w", err)
	}

	return buf.Bytes(), nil
}

func lerp(a, b uint8, t float64) uint8 {
	return uint8(float64(a) + (float64(b)-float64(a))*t)
}
