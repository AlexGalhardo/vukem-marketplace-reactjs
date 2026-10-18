package seeders

import (
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/factory"
	"github.com/AlexGalhardo/vukem/orm"
	"github.com/AlexGalhardo/vukem/uuid"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/catalog"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/models"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/payments"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/qna"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/reviews"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/shipping"
)

// DemoBuyerDomain is the email domain of every demo buyer; with
// DemoSellerDomain it identifies the demo catalog's rows.
const DemoBuyerDomain = "compradores.example.com"

// Demo activity defaults: 300 buyers with 1–9 orders each (5 on average,
// 1500 orders) and 0–8 product questions each (4 on average, 1200).
const (
	DefaultDemoBuyers       = 300
	DefaultDemoMaxOrders    = 9
	DefaultDemoMaxQuestions = 8
)

// demoHistory is how far back the simulated orders and questions go.
const demoHistory = 180 * 24 * time.Hour

// buyerChunk buyers, with everything they did, are written per transaction.
const buyerChunk = 50

// The rows below are the real commerce tables. The demo activity is written
// with plain inserts and never through the payment, review or question
// services: no provider call, email, webhook, realtime event or queued job
// can follow. The rows are demo rows because their buyer's email ends in
// DemoBuyerDomain and their shop's owner's in DemoSellerDomain.
//
// No ledger_entries are written: the ledger is money that really moved
// (balances and payouts sum it), and none did. Demo shops therefore show
// sales but a zero balance, and nothing can ever be paid out for them.

type demoOrderRow struct {
	orm.Model

	PublicID        string       `db:"public_id"`
	BuyerID         int64        `db:"buyer_id"`
	Status          string       `db:"status"`
	PaymentMethod   string       `db:"payment_method"`
	SubtotalCents   int64        `db:"subtotal_cents"`
	ShippingCents   int64        `db:"shipping_cents"`
	FeeCents        int64        `db:"fee_cents"`
	TotalCents      int64        `db:"total_cents"`
	ShippingAddress string       `db:"shipping_address"`
	PaidAt          orm.NullTime `db:"paid_at"`
}

func (demoOrderRow) Table() string { return "orders" }

type demoSellerOrderRow struct {
	orm.Model

	OrderID       int64            `db:"order_id"`
	ShopID        int64            `db:"shop_id"`
	Status        string           `db:"status"`
	SubtotalCents int64            `db:"subtotal_cents"`
	ShippingCents int64            `db:"shipping_cents"`
	FeeCents      int64            `db:"fee_cents"`
	PayoutCents   int64            `db:"payout_cents"`
	Carrier       sql.Null[string] `db:"carrier"`
	TrackingCode  sql.Null[string] `db:"tracking_code"`
	ShippedAt     orm.NullTime     `db:"shipped_at"`
	DeliveredAt   orm.NullTime     `db:"delivered_at"`
}

func (demoSellerOrderRow) Table() string { return "seller_orders" }

type demoOrderItemRow struct {
	orm.Model

	SellerOrderID int64  `db:"seller_order_id"`
	ProductID     int64  `db:"product_id"`
	Title         string `db:"title"`
	PriceCents    int64  `db:"price_cents"`
	Quantity      int    `db:"quantity"`
	WeightGrams   int    `db:"weight_grams"`
}

func (demoOrderItemRow) Table() string { return "order_items" }

type demoReviewRow struct {
	orm.Model

	ProductID       int64            `db:"product_id"`
	BuyerID         int64            `db:"buyer_id"`
	OrderItemID     int64            `db:"order_item_id"`
	Rating          int              `db:"rating"`
	Title           sql.Null[string] `db:"title"`
	Body            sql.Null[string] `db:"body"`
	Recommend       sql.Null[bool]   `db:"recommend"`
	SellerReply     sql.Null[string] `db:"seller_reply"`
	SellerRepliedAt orm.NullTime     `db:"seller_replied_at"`
	HelpfulCount    int              `db:"helpful_count"`
}

func (demoReviewRow) Table() string { return "reviews" }

type demoVoteRow struct {
	ID        int64     `db:"id"`
	ReviewID  int64     `db:"review_id"`
	UserID    int64     `db:"user_id"`
	CreatedAt time.Time `db:"created_at"`
}

func (demoVoteRow) Table() string { return "review_votes" }

type demoAddressRow struct {
	orm.Model

	UserID     int64            `db:"user_id"`
	Recipient  string           `db:"recipient"`
	CEP        string           `db:"cep"`
	Street     string           `db:"street"`
	Number     string           `db:"number"`
	Complement sql.Null[string] `db:"complement"`
	District   string           `db:"district"`
	City       string           `db:"city"`
	UF         string           `db:"uf"`
	IsDefault  bool             `db:"is_default"`
}

func (demoAddressRow) Table() string { return "addresses" }

// addressSnapshot is orders.shipping_address, as checkout.Place writes it.
type addressSnapshot struct {
	Recipient  string `json:"recipient"`
	CEP        string `json:"cep"`
	Street     string `json:"street"`
	Number     string `json:"number"`
	Complement string `json:"complement,omitempty"`
	District   string `json:"district"`
	City       string `json:"city"`
	UF         string `json:"uf"`
}

// demoActivityResult reports what the activity part of a run created.
type demoActivityResult struct {
	Buyers, Orders, Reviews, Questions int
}

// plannedBuyer is one demo buyer and their address, ids not yet set.
type plannedBuyer struct {
	user models.User
	addr demoAddressRow
}

// demoListing is a demo product as the activity planner needs it.
type demoListing struct {
	id, shopID, ownerID int64
	slug, title         string
	price               int64
	weight, stock       int
}

// trackingLetters make the two-letter prefix of the simulated tracking codes.
var trackingLetters = strings.Split("ABCDEFGHIJKLMNOPQRSTUVWXYZ", "")

// buyerActivity is everything one buyer did, ready to insert.
type buyerActivity struct {
	orders    []plannedOrder
	questions []qna.Question
}

type plannedOrder struct {
	order   demoOrderRow
	sellers []plannedSellerOrder
}

type plannedSellerOrder struct {
	so    demoSellerOrderRow
	lines []plannedLine
}

type plannedLine struct {
	item   demoOrderItemRow
	review *plannedReview
}

type plannedReview struct {
	row   demoReviewRow
	votes []demoVoteRow
}

// planBuyers builds the demo buyers. It depends only on the seed (and now,
// for the verification time), so a re-run plans the same people.
func (d DemoCatalog) planBuyers(now time.Time) []plannedBuyer {
	var cities []int

	for i, c := range buyerCities {
		for range c.weightPick {
			cities = append(cities, i)
		}
	}

	return factory.New(func(f *factory.Faker) plannedBuyer {
		first, last := factory.Pick(f, buyerFirstNames...), factory.Pick(f, buyerLastNames...)
		name := first + " " + last
		local := strings.ReplaceAll(catalog.Slugify(name), "-", ".")
		city := buyerCities[factory.Pick(f, cities...)]

		u := models.User{
			Name:  name,
			Email: fmt.Sprintf("%s.%03d@%s", local, f.Seq(), DemoBuyerDomain),
			Role:  models.RoleUser,
		}
		u.EmailVerifiedAt.V, u.EmailVerifiedAt.Valid = now, true

		complement := factory.Pick(f, buyerComplements...)

		return plannedBuyer{user: u, addr: demoAddressRow{
			Recipient:  name,
			CEP:        fmt.Sprintf("%05d%03d", f.Int(city.cepFrom, city.cepTo), f.Int(0, 999)),
			Street:     factory.Pick(f, buyerStreets...),
			Number:     strconv.Itoa(f.Int(1, 2400)),
			Complement: sql.Null[string]{V: complement, Valid: complement != ""},
			District:   factory.Pick(f, city.districts...),
			City:       city.name,
			UF:         city.uf,
			IsDefault:  true,
		}}
	}).Seed(d.Seed ^ 0xb0e5b0e5).Count(d.Buyers).Make()
}

// seedActivity creates the missing demo buyers, then the orders, reviews
// and questions of every demo buyer that has none yet. A buyer's activity
// is written in one transaction, so a buyer has either all of it or none.
func (d DemoCatalog) seedActivity(ctx context.Context, db *database.DB, now time.Time) (demoActivityResult, error) {
	res := demoActivityResult{}

	listings, err := loadListings(ctx, db)
	if err != nil || len(listings) == 0 {
		return res, err
	}

	rates, err := (shipping.Freight{DB: db}).Rates(ctx)
	if err != nil {
		return res, fmt.Errorf("load freight rates: %w", err)
	}

	if len(rates) == 0 {
		rates = shipping.DefaultRates()
	}

	plan := d.planBuyers(now)

	if res.Buyers, err = d.insertMissingBuyers(ctx, db, plan); err != nil {
		return res, err
	}

	ids, done, err := buyerState(ctx, db, plan)
	if err != nil {
		return res, err
	}

	byShop := map[int64][]int{}
	for i := range listings {
		byShop[listings[i].shopID] = append(byShop[listings[i].shopID], i)
	}

	p := activityPlanner{
		d: d, now: now, listings: listings, byShop: byShop, rates: rates, buyerIDs: ids,
	}

	var todo []int

	for i := range plan {
		if !done[ids[i]] {
			todo = append(todo, i)
		}
	}

	for chunk := range slicesChunk(todo, buyerChunk) {
		acts := make([]buyerActivity, len(chunk))
		stock := map[int]int{}

		for k, b := range chunk {
			acts[k] = p.plan(b, plan[b].addr, stock)
		}

		if err := database.RunInTx(ctx, db, func(tx *database.Tx) error {
			return insertActivity(ctx, tx, acts, listings, stock)
		}); err != nil {
			return res, fmt.Errorf("seed demo activity: %w", err)
		}

		for k := range acts {
			res.Orders += len(acts[k].orders)
			res.Questions += len(acts[k].questions)
		}

		eachLine(acts, func(_ *plannedSellerOrder, l *plannedLine) {
			if l.review != nil {
				res.Reviews++
			}
		})
	}

	return res, nil
}

// loadListings returns the published products of active demo shops in
// slug order: the planner's product index is stable across databases.
func loadListings(ctx context.Context, db *database.DB) ([]demoListing, error) {
	rows, err := db.QueryContext(ctx, database.Rebind(db.Dialect(),
		`SELECT p.id, p.shop_id, s.owner_id, p.slug, p.title, p.price_cents, p.weight_grams, p.stock
		FROM products p JOIN shops s ON s.id = p.shop_id JOIN users u ON u.id = s.owner_id
		WHERE u.email LIKE ? AND p.status = ? AND s.status = ?`),
		"%@"+DemoSellerDomain, catalog.StatusPublished, catalog.ShopActive)
	if err != nil {
		return nil, fmt.Errorf("load demo products: %w", err)
	}
	defer rows.Close()

	var out []demoListing

	for rows.Next() {
		var l demoListing
		if err := rows.Scan(&l.id, &l.shopID, &l.ownerID, &l.slug, &l.title, &l.price, &l.weight,
			&l.stock); err != nil {
			return nil, fmt.Errorf("scan demo product: %w", err)
		}

		out = append(out, l)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("load demo products: %w", err)
	}

	slices.SortFunc(out, func(a, b demoListing) int { return strings.Compare(a.slug, b.slug) })

	return out, nil
}

// insertMissingBuyers writes the planned buyers whose email is not stored
// yet, with their address.
func (d DemoCatalog) insertMissingBuyers(ctx context.Context, db *database.DB, plan []plannedBuyer) (int, error) {
	emails := make([]any, len(plan))
	for i := range plan {
		emails[i] = plan[i].user.Email
	}

	have, err := orm.QueryOf[models.User](db).WhereIn("email", emails...).Pluck[string](ctx, "email")
	if err != nil {
		return 0, fmt.Errorf("load demo buyers: %w", err)
	}

	seen := make(map[string]bool, len(have))
	for _, e := range have {
		seen[e] = true
	}

	var todo []plannedBuyer

	for i := range plan {
		if !seen[plan[i].user.Email] {
			todo = append(todo, plan[i])
		}
	}

	users := make([]*models.User, len(todo))
	for i := range todo {
		users[i] = &todo[i].user
	}

	if err := d.hashPasswords(users); err != nil {
		return 0, err
	}

	for chunk := range slicesChunk(todo, buyerChunk) {
		if err := database.RunInTx(ctx, db, func(tx *database.Tx) error {
			us := make([]models.User, len(chunk))
			for i := range chunk {
				us[i] = chunk[i].user
			}

			if err := orm.CreateMany(ctx, tx, us); err != nil {
				return fmt.Errorf("insert buyers: %w", err)
			}

			addrs := make([]demoAddressRow, len(chunk))
			for i := range chunk {
				addrs[i] = chunk[i].addr
				addrs[i].UserID = us[i].ID
			}

			if err := orm.CreateMany(ctx, tx, addrs); err != nil {
				return fmt.Errorf("insert buyer addresses: %w", err)
			}

			return nil
		}); err != nil {
			return 0, fmt.Errorf("seed demo buyers: %w", err)
		}
	}

	return len(todo), nil
}

// buyerState returns the id of every planned buyer (by plan index) and
// which of them already have activity (an order or a question): those are
// skipped, so a re-run never duplicates.
func buyerState(ctx context.Context, db *database.DB, plan []plannedBuyer) ([]int64, map[int64]bool, error) {
	users, err := orm.QueryOf[models.User](db).Select("id", "email").
		Where("email", "LIKE", "%@"+DemoBuyerDomain).Get(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("load demo buyers: %w", err)
	}

	byEmail := make(map[string]int64, len(users))
	for i := range users {
		byEmail[users[i].Email] = users[i].ID
	}

	ids := make([]int64, len(plan))

	for i := range plan {
		id, ok := byEmail[plan[i].user.Email]
		if !ok {
			return nil, nil, fmt.Errorf("demo buyer %s is missing", plan[i].user.Email)
		}

		ids[i] = id
	}

	rows, err := db.QueryContext(ctx, database.Rebind(db.Dialect(),
		`SELECT o.buyer_id FROM orders o JOIN users u ON u.id = o.buyer_id WHERE u.email LIKE ?
		UNION SELECT q.asker_id FROM product_questions q JOIN users u ON u.id = q.asker_id WHERE u.email LIKE ?`),
		"%@"+DemoBuyerDomain, "%@"+DemoBuyerDomain)
	if err != nil {
		return nil, nil, fmt.Errorf("load demo buyer activity: %w", err)
	}
	defer rows.Close()

	done := map[int64]bool{}

	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, nil, err
		}

		done[id] = true
	}

	return ids, done, rows.Err()
}

// activityPlanner plans one buyer's orders, reviews and questions.
type activityPlanner struct {
	d        DemoCatalog
	now      time.Time
	listings []demoListing
	byShop   map[int64][]int
	rates    []shipping.Rate
	buyerIDs []int64
}

// pick returns a listing index skewed to the head of the list: a few
// products sell (and get reviewed and asked about) a lot, most a little.
func (p *activityPlanner) pick(f *factory.Faker) int {
	u := f.Float(0, 1)
	return min(int(u*u*float64(len(p.listings))), len(p.listings)-1)
}

// within returns a time between from and to (from when to is not after it).
func within(f *factory.Faker, from, to time.Time) time.Time {
	span := to.Sub(from)
	if span <= 0 {
		return from
	}

	return from.Add(time.Duration(f.Float(0, 1) * float64(span))).Truncate(time.Second)
}

func minutes(f *factory.Faker, lo, hi int) time.Duration {
	return time.Duration(f.Int(lo, hi)) * time.Minute
}

// plan builds buyer b's activity. stock carries the running stock delta of
// the chunk per listing index (sales minus restocks), starting from the
// listing's stored stock.
func (p *activityPlanner) plan(b int, addr demoAddressRow, stock map[int]int) buyerActivity {
	f := factory.NewFaker(p.d.Seed ^ uint64(b+1)<<24 ^ 0xac71) //nolint:gosec // G115: b is a small non-negative index
	nOrders := f.Int(1, p.d.MaxOrders)
	nQuestions := f.Int(0, p.d.MaxQuestions)
	act := buyerActivity{}
	reviewed := map[int]bool{}

	snapshot, _ := json.Marshal(addressSnapshot{ //nolint:errchkjson // strings only: it always marshals
		Recipient: addr.Recipient, CEP: addr.CEP, Street: addr.Street, Number: addr.Number,
		Complement: addr.Complement.V, District: addr.District, City: addr.City, UF: addr.UF,
	})

	region, err := shipping.RegionOf(addr.UF)
	if err != nil {
		region = shipping.Southeast
	}

	for range nOrders {
		act.orders = append(act.orders, p.order(f, b, string(snapshot), region, stock, reviewed))
	}

	for range nQuestions {
		if q, ok := p.question(f, b); ok {
			act.questions = append(act.questions, q)
		}
	}

	return act
}

// order plans one order: 1–3 lines, sometimes from several shops.
func (p *activityPlanner) order(f *factory.Faker, b int, snapshot string, region shipping.Region,
	stock map[int]int, reviewed map[int]bool,
) plannedOrder {
	created := p.now.Add(-minutes(f, 60, int(demoHistory/time.Minute))).Truncate(time.Second)
	canceled := f.Int(1, 100) <= 6
	paidAt := created.Add(minutes(f, 1, 30))

	method := payments.MethodPix
	if f.Int(1, 100) <= 45 {
		method = payments.MethodCard
	}

	nLines := 1
	if r := f.Int(1, 100); r > 90 {
		nLines = 3
	} else if r > 70 {
		nLines = 2
	}

	first := p.pick(f)
	picked := []int{first}

	for len(picked) < nLines {
		var next int

		if same := p.byShop[p.listings[first].shopID]; f.Bool() && len(same) > 1 {
			next = factory.Pick(f, same...)
		} else {
			next = p.pick(f)
		}

		if slices.Contains(picked, next) {
			break
		}

		picked = append(picked, next)
	}

	var shops []int64

	groups := map[int64][]int{}

	for _, li := range picked {
		s := p.listings[li].shopID
		if _, ok := groups[s]; !ok {
			shops = append(shops, s)
		}

		groups[s] = append(groups[s], li)
	}

	o := plannedOrder{order: demoOrderRow{
		PublicID: publicIDAt(created), BuyerID: p.buyerIDs[b], PaymentMethod: method,
		ShippingAddress: snapshot, Status: payments.OrderPaid,
	}}
	o.order.CreatedAt, o.order.UpdatedAt = created, created

	if canceled {
		o.order.Status = payments.OrderCanceled
		o.order.UpdatedAt = created.Add(minutes(f, 30, 24*60))
	} else {
		o.order.PaidAt.V, o.order.PaidAt.Valid = paidAt, true
		o.order.UpdatedAt = paidAt
	}

	for _, shopID := range shops {
		so := p.sellerOrder(f, b, shopID, groups[shopID], created, paidAt, canceled, region, stock, reviewed)
		so.so.CreatedAt = created
		o.order.UpdatedAt = maxTime(o.order.UpdatedAt, so.so.UpdatedAt)
		o.order.SubtotalCents += so.so.SubtotalCents
		o.order.ShippingCents += so.so.ShippingCents
		o.order.FeeCents += so.so.FeeCents
		o.sellers = append(o.sellers, so)
	}

	o.order.TotalCents = o.order.SubtotalCents + o.order.ShippingCents

	return o
}

func maxTime(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}

	return a
}

// sellerOrder plans one shop's part of an order: its lines, money split,
// shipping status by age and the reviews of its lines.
func (p *activityPlanner) sellerOrder(f *factory.Faker, b int, shopID int64, lines []int,
	created, paidAt time.Time, canceled bool, region shipping.Region, stock map[int]int, reviewed map[int]bool,
) plannedSellerOrder {
	so := plannedSellerOrder{so: demoSellerOrderRow{ShopID: shopID, Status: payments.SellerCanceled}}
	weight := 0

	for _, li := range lines {
		l := p.listings[li]

		qty := 1
		if r := f.Int(1, 100); r > 95 {
			qty = 3
		} else if r > 80 {
			qty = 2
		}

		if !canceled {
			if have := l.stock + stock[li]; have < qty {
				stock[li] += qty - have + f.Int(10, 50) // the seller restocked
			}

			stock[li] -= qty
		}

		so.so.SubtotalCents += l.price * int64(qty)
		weight += l.weight * qty
		so.lines = append(so.lines, plannedLine{item: demoOrderItemRow{
			ProductID: l.id, Title: l.title, PriceCents: l.price, Quantity: qty, WeightGrams: l.weight,
		}})
	}

	q, err := shipping.QuoteFrom(p.rates, region, weight)
	if err != nil {
		q = shipping.Quote{PriceCents: 0, Days: 7}
	}

	split := payments.SplitOf(so.so.SubtotalCents, q.PriceCents, p.d.FeeBPS)
	so.so.ShippingCents, so.so.FeeCents, so.so.PayoutCents = split.ShippingCents, split.FeeCents, split.PayoutCents
	so.so.UpdatedAt = created

	if !canceled {
		p.advance(f, &so.so, paidAt, q.Days)
	}

	for i := range so.lines {
		li := lines[i]
		item := &so.lines[i].item
		item.CreatedAt, item.UpdatedAt = created, created

		if canceled || reviewed[li] {
			continue
		}

		if r := p.review(f, b, li, &so.so, paidAt); r != nil {
			reviewed[li] = true
			so.lines[i].review = r
		}
	}

	return so
}

// advance moves a paid seller order along by its age: paid for the first
// days, then shipped, then delivered after the freight days.
func (p *activityPlanner) advance(f *factory.Faker, so *demoSellerOrderRow, paidAt time.Time, days int) {
	so.Status, so.UpdatedAt = payments.SellerPaid, paidAt

	shipped := paidAt.Add(minutes(f, 4*60, 48*60))
	if shipped.After(p.now) {
		return
	}

	so.Status, so.UpdatedAt = payments.SellerShipped, shipped
	so.ShippedAt.V, so.ShippedAt.Valid = shipped, true
	so.Carrier = sql.Null[string]{V: factory.Pick(f, demoCarriers...), Valid: true}
	so.TrackingCode = sql.Null[string]{
		V: fmt.Sprintf("%s%s%09dBR", factory.Pick(f, trackingLetters...), factory.Pick(f, trackingLetters...),
			f.Int(0, 999_999_999)),
		Valid: true,
	}

	delivered := shipped.Add(time.Duration(days+f.Int(-1, 2))*24*time.Hour + minutes(f, 0, 600))
	if delivered.After(p.now) || f.Int(1, 100) <= 3 { // a few parcels are still on the way
		return
	}

	so.Status, so.UpdatedAt = payments.SellerDelivered, delivered
	so.DeliveredAt.V, so.DeliveredAt.Valid = delivered, true
}

// review plans the buyer's review of listing li, or nil: most delivered
// lines are reviewed, few that are only paid or shipped. Ratings lean to 4
// and 5 stars.
func (p *activityPlanner) review(f *factory.Faker, b, li int, so *demoSellerOrderRow, paidAt time.Time) *plannedReview {
	var at time.Time

	switch so.Status {
	case payments.SellerDelivered:
		if f.Int(1, 100) > 65 {
			return nil
		}

		at = so.DeliveredAt.V.Add(minutes(f, 120, 15*24*60))
	case payments.SellerShipped, payments.SellerPaid:
		if f.Int(1, 100) > 8 {
			return nil
		}

		at = paidAt.Add(minutes(f, 60, 48*60))
	default:
		return nil
	}

	if at.After(p.now) {
		return nil
	}

	var rating int

	switch r := f.Int(1, 100); {
	case r <= 50:
		rating = 5
	case r <= 78:
		rating = 4
	case r <= 88:
		rating = 3
	case r <= 93:
		rating = 2
	default:
		rating = 1
	}

	l := p.listings[li]
	rv := demoReviewRow{ProductID: l.id, BuyerID: p.buyerIDs[b], Rating: rating}
	rv.CreatedAt, rv.UpdatedAt = at, at

	if f.Int(1, 100) <= 85 {
		rv.Title = sql.Null[string]{V: factory.Pick(f, reviewTitles[rating-1]...), Valid: true}
	}

	if f.Int(1, 100) <= 90 {
		rv.Body = sql.Null[string]{V: factory.Pick(f, reviewBodies[rating-1]...), Valid: true}
	}

	switch r := f.Int(1, 100); {
	case rating >= 4 && r <= 90:
		rv.Recommend = sql.Null[bool]{V: true, Valid: true}
	case rating == 3 && r <= 50:
		rv.Recommend = sql.Null[bool]{V: r <= 25, Valid: true}
	case rating <= 2 && r <= 80:
		rv.Recommend = sql.Null[bool]{V: false, Valid: true}
	}

	replyChance := 15
	if rating <= 2 {
		replyChance = 60
	} else if rating == 3 {
		replyChance = 25
	}

	if f.Int(1, 100) <= replyChance {
		if replied := at.Add(minutes(f, 180, 72*60)); !replied.After(p.now) {
			replies := sellerRepliesPositive
			if rating <= 3 {
				replies = sellerRepliesNegative
			}

			rv.SellerReply = sql.Null[string]{V: factory.Pick(f, replies...), Valid: true}
			rv.SellerRepliedAt.V, rv.SellerRepliedAt.Valid = replied, true
			rv.UpdatedAt = replied
		}
	}

	out := &plannedReview{row: rv}

	var nVotes int

	switch r := f.Int(1, 100); {
	case r <= 45:
	case r <= 85:
		nVotes = f.Int(1, 3)
	default:
		nVotes = f.Int(4, 12)
	}

	voted := map[int]bool{b: true}

	for range nVotes {
		v := f.Int(0, len(p.buyerIDs)-1)
		if voted[v] {
			continue
		}

		voted[v] = true
		out.votes = append(out.votes, demoVoteRow{UserID: p.buyerIDs[v], CreatedAt: within(f, at, p.now)})
	}

	out.row.HelpfulCount = len(out.votes)

	return out
}

// question plans one question of buyer b on a demo product; most are
// answered by the shop within two days.
func (p *activityPlanner) question(f *factory.Faker, b int) (qna.Question, bool) {
	l := p.listings[p.pick(f)]
	if l.ownerID == p.buyerIDs[b] {
		return qna.Question{}, false
	}

	qa := factory.Pick(f, demoQuestions...)
	created := p.now.Add(-minutes(f, 10, int(demoHistory/time.Minute))).Truncate(time.Second)
	q := qna.Question{ProductID: l.id, ShopID: l.shopID, AskerID: p.buyerIDs[b], Body: qa.question}
	q.CreatedAt, q.UpdatedAt = created, created

	if f.Int(1, 100) <= 85 {
		if answered := created.Add(minutes(f, 20, 48*60)); !answered.After(p.now) {
			q.Answer = sql.Null[string]{V: factory.Pick(f, qa.answers...), Valid: true}
			q.AnsweredAt.V, q.AnsweredAt.Valid = answered, true
			q.UpdatedAt = answered
		}
	}

	return q, true
}

// publicIDAt is a UUIDv7 (payments.NewPublicID) whose timestamp is the
// simulated order time instead of the seeding time.
func publicIDAt(t time.Time) string {
	id := uuid.NewV7()
	var ms [8]byte
	binary.BigEndian.PutUint64(ms[:], uint64(max(t.UnixMilli(), 0)))
	copy(id[:6], ms[2:])

	return id.String()
}

// insertActivity writes a chunk of buyers' activity: one multi-row INSERT
// per table, then the stock changes and the ratings of the reviewed
// products.
func insertActivity(ctx context.Context, tx *database.Tx, acts []buyerActivity, listings []demoListing,
	stock map[int]int,
) error {
	var orders []demoOrderRow

	for k := range acts {
		for i := range acts[k].orders {
			orders = append(orders, acts[k].orders[i].order)
		}
	}

	if err := orm.CreateMany(ctx, tx, orders); err != nil {
		return fmt.Errorf("insert orders: %w", err)
	}

	var sos []demoSellerOrderRow

	n := 0

	for k := range acts {
		for i := range acts[k].orders {
			for j := range acts[k].orders[i].sellers {
				so := acts[k].orders[i].sellers[j].so
				so.OrderID = orders[n].ID
				sos = append(sos, so)
			}

			n++
		}
	}

	if err := orm.CreateMany(ctx, tx, sos); err != nil {
		return fmt.Errorf("insert seller orders: %w", err)
	}

	var (
		items []demoOrderItemRow
		lines []*plannedLine
	)

	n = 0

	eachSellerOrder(acts, func(so *plannedSellerOrder) {
		for i := range so.lines {
			item := so.lines[i].item
			item.SellerOrderID = sos[n].ID
			items = append(items, item)
			lines = append(lines, &so.lines[i])
		}

		n++
	})

	if err := orm.CreateMany(ctx, tx, items); err != nil {
		return fmt.Errorf("insert order items: %w", err)
	}

	var (
		revs    []demoReviewRow
		planned []*plannedReview
	)

	for i, l := range lines {
		if l.review != nil {
			r := l.review.row
			r.OrderItemID = items[i].ID
			revs = append(revs, r)
			planned = append(planned, l.review)
		}
	}

	if err := orm.CreateMany(ctx, tx, revs); err != nil {
		return fmt.Errorf("insert reviews: %w", err)
	}

	var (
		votes    []demoVoteRow
		reviewed []int64
	)

	for i, r := range planned {
		reviewed = append(reviewed, revs[i].ProductID)

		for _, v := range r.votes {
			v.ReviewID = revs[i].ID
			votes = append(votes, v)
		}
	}

	if err := orm.CreateMany(ctx, tx, votes); err != nil {
		return fmt.Errorf("insert review votes: %w", err)
	}

	var questions []qna.Question
	for k := range acts {
		questions = append(questions, acts[k].questions...)
	}

	if err := qna.InsertDemo(ctx, tx, questions); err != nil {
		return err
	}

	if err := applyStock(ctx, tx, listings, stock); err != nil {
		return err
	}

	slices.Sort(reviewed)

	return reviews.Recalculate(ctx, tx, slices.Compact(reviewed)...)
}

// eachSellerOrder calls fn with every planned seller order of acts, in
// insertion order.
func eachSellerOrder(acts []buyerActivity, fn func(so *plannedSellerOrder)) {
	for k := range acts {
		for i := range acts[k].orders {
			for j := range acts[k].orders[i].sellers {
				fn(&acts[k].orders[i].sellers[j])
			}
		}
	}
}

// eachLine calls fn with every planned order line of acts.
func eachLine(acts []buyerActivity, fn func(so *plannedSellerOrder, l *plannedLine)) {
	eachSellerOrder(acts, func(so *plannedSellerOrder) {
		for i := range so.lines {
			fn(so, &so.lines[i])
		}
	})
}

// stockBatch bounds the CASE arms of one stock UPDATE.
const stockBatch = 200

// applyStock adds each listing's stock delta (restocks minus sales) and
// keeps the in-memory listings in step for the next chunk. The planner
// restocks before a sale would go below zero.
func applyStock(ctx context.Context, tx *database.Tx, listings []demoListing, stock map[int]int) error {
	idx := make([]int, 0, len(stock))

	for li, delta := range stock {
		if delta != 0 {
			idx = append(idx, li)
		}
	}

	slices.Sort(idx)

	for batch := range slices.Chunk(idx, stockBatch) {
		var (
			b    strings.Builder
			args []any
			in   []any
		)

		b.WriteString(`UPDATE products SET stock = stock + CASE id`)

		for _, li := range batch {
			b.WriteString(` WHEN ? THEN CAST(? AS INTEGER)`)

			args = append(args, listings[li].id, stock[li])
			in = append(in, listings[li].id)
		}

		b.WriteString(` END WHERE id IN (` + strings.TrimSuffix(strings.Repeat("?, ", len(batch)), ", ") + `)`)

		q, err := database.RebindChecked(tx.Dialect(), b.String())
		if err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, q, append(args, in...)...); err != nil {
			return fmt.Errorf("update demo stock: %w", err)
		}
	}

	for li, delta := range stock {
		listings[li].stock += delta
	}

	return nil
}
