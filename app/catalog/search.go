package catalog

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/AlexGalhardo/vukem/cache"
	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/z"
)

// Page is one page of results.
type Page[T any] struct {
	Items    []T   `json:"data"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PerPage  int   `json:"per_page"`
	LastPage int   `json:"last_page"`
}

// ProductCard is a product in a list (search, category, storefront, home).
type ProductCard struct {
	ID         int64     `json:"id"`
	Slug       string    `json:"slug"`
	Title      string    `json:"title"`
	PriceCents int64     `json:"price_cents"`
	Stock      int       `json:"stock"`
	ShopSlug   string    `json:"shop_slug"`
	ShopName   string    `json:"shop_name"`
	ShipFromUF string    `json:"ship_from_uf"`
	ImageURL   string    `json:"image_url"`
	CreatedAt  time.Time `json:"created_at"`
	// RatingAvgX100 and RatingCount drive the stars on product cards.
	RatingAvgX100 int `json:"rating_avg_x100"`
	RatingCount   int `json:"rating_count"`
}

// Sort orders.
const (
	SortRelevance = "relevance"
	SortPriceAsc  = "price_asc"
	SortPriceDesc = "price_desc"
	SortNewest    = "newest"
)

// Query is a validated search request (see QuerySchema). Zero values mean
// "no filter".
type Query struct {
	Q        string `json:"q"        form:"q"`
	Category string `json:"category" form:"category"`
	Shop     string `json:"shop"     form:"shop"`
	MinCents int64  `json:"min"      form:"min"`
	MaxCents int64  `json:"max"      form:"max"`
	UF       string `json:"uf"       form:"uf"`
	Sort     string `json:"sort"     form:"sort"`
	Page     int    `json:"page"     form:"page"`
	PerPage  int    `json:"per_page" form:"per_page"`
}

// MaxPerPage caps page sizes (pages and APIs).
const MaxPerPage = 48

// QuerySchema validates a search request from a query string.
var QuerySchema = z.Struct[Query](z.Shape{
	"q":        z.String().Trim().Max(MaxSearchTermLen).Optional(),
	"category": z.String().Trim().Max(64).Optional(),
	"shop":     z.String().Trim().Max(80).Optional(),
	"min":      z.Int64().Min(0).Max(MaxPriceCents).Optional(),
	"max":      z.Int64().Min(0).Max(MaxPriceCents).Optional(),
	"uf":       z.String().Trim().ToUpper().OneOf(append([]string{""}, UFs...)...).Optional(),
	"sort":     z.String().OneOf("", SortRelevance, SortPriceAsc, SortPriceDesc, SortNewest).Optional(),
	"page":     z.Int().Min(1).Max(1000).Optional(),
	"per_page": z.Int().Min(1).Max(MaxPerPage).Optional(),
})

// Normalized applies the defaults: page 1, 24 per page, relevance (newest
// without a term), collapsed spaces in the term.
func (q Query) Normalized() Query {
	q.Q = strings.Join(strings.Fields(q.Q), " ")
	q.Page = max(q.Page, 1)

	if q.PerPage <= 0 {
		q.PerPage = 24
	}

	q.PerPage = min(q.PerPage, MaxPerPage)

	if q.Sort == "" {
		q.Sort = SortRelevance
	}

	if q.Sort == SortRelevance && q.Q == "" {
		q.Sort = SortNewest
	}

	return q
}

// searchCacheTTL bounds how stale an anonymous result page can be.
const searchCacheTTL = 45 * time.Second

// Search returns visible products matching q. Anonymous callers share a
// short-lived cache (cached = true): signed-in pages may later carry
// per-user data, so they always hit the database.
func (s *Service) Search(ctx context.Context, q Query, cached bool) (Page[ProductCard], error) {
	q = q.Normalized()

	if !cached || s.cache == nil {
		return s.search(ctx, q)
	}

	b, err := json.Marshal(q)
	if err != nil {
		return Page[ProductCard]{}, fmt.Errorf("search cache key: %w", err)
	}

	sum := sha256.Sum256(b)

	return cache.Remember(ctx, s.cache, "catalog:search:"+hex.EncodeToString(sum[:16]), searchCacheTTL,
		func(ctx context.Context) (Page[ProductCard], error) { return s.search(ctx, q) })
}

// firstImageSQL selects the key and width (0 = not processed yet) of a
// product's first image (alias "p").
const firstImageSQL = `(SELECT i.key FROM product_images i WHERE i.product_id = p.id ORDER BY i.position, i.id LIMIT 1),
	(SELECT i.width FROM product_images i WHERE i.product_id = p.id ORDER BY i.position, i.id LIMIT 1)`

// How the search term matches.
type matchMode int

const (
	matchNone     matchMode = iota // no term
	matchFullText                  // Postgres websearch_to_tsquery over the tsvector (GIN)
	matchTrigram                   // Postgres word similarity on the title (GIN trigram)
	matchLike                      // SQLite substring (unit tests)
)

// search runs full text first and falls back to trigram word similarity on
// the title only when full text finds nothing (typos, partial words), so a
// precise query is never diluted by fuzzy matches.
func (s *Service) search(ctx context.Context, q Query) (Page[ProductCard], error) {
	switch {
	case q.Q == "":
		return s.searchWith(ctx, q, matchNone)
	case !s.isPostgres():
		return s.searchWith(ctx, q, matchLike)
	}

	page, err := s.searchWith(ctx, q, matchFullText)
	if err != nil || page.Total > 0 {
		return page, err
	}

	return s.searchWith(ctx, q, matchTrigram)
}

func (s *Service) searchWith(ctx context.Context, q Query, mode matchMode) (Page[ProductCard], error) {
	b := newSQL(s.db.Dialect())
	where := []string{visibleSQL}

	switch mode {
	case matchFullText:
		where = append(where, `p.search @@ websearch_to_tsquery('simple', `+b.arg(q.Q)+`)`)
	case matchTrigram:
		where = append(where, b.arg(q.Q)+` <% p.title`)
	case matchLike:
		like := "%" + escapeLike(q.Q) + "%"
		where = append(
			where,
			`(p.title LIKE `+b.arg(like)+` ESCAPE '\' OR p.description LIKE `+b.arg(like)+` ESCAPE '\')`,
		)
	case matchNone:
	}

	if q.Category != "" {
		where = append(where, `p.category_id IN (SELECT id FROM categories WHERE slug = `+b.arg(q.Category)+
			` OR parent_id IN (SELECT id FROM categories WHERE slug = `+b.arg(q.Category)+`))`)
	}

	if q.Shop != "" {
		where = append(where, `s.slug = `+b.arg(q.Shop))
	}

	if q.MinCents > 0 {
		where = append(where, `p.price_cents >= `+b.arg(q.MinCents))
	}

	if q.MaxCents > 0 {
		where = append(where, `p.price_cents <= `+b.arg(q.MaxCents))
	}

	if q.UF != "" {
		where = append(where, `s.ship_from_uf = `+b.arg(q.UF))
	}

	from := ` FROM products p JOIN shops s ON s.id = p.shop_id WHERE ` + strings.Join(where, " AND ")

	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*)`+from, b.args...).Scan(&total); err != nil {
		return Page[ProductCard]{}, fmt.Errorf("count search: %w", err)
	}

	var order string

	switch q.Sort {
	case SortPriceAsc:
		order = `p.price_cents ASC, p.id DESC`
	case SortPriceDesc:
		order = `p.price_cents DESC, p.id DESC`
	case SortRelevance:
		// Placeholders added after the COUNT query, which does not use them.
		switch mode {
		case matchFullText:
			order = `ts_rank(p.search, websearch_to_tsquery('simple', ` + b.arg(q.Q) + `)) DESC, p.id DESC`
		case matchTrigram:
			order = `word_similarity(` + b.arg(q.Q) + `, p.title) DESC, p.id DESC`
		case matchNone, matchLike:
			order = `p.id DESC`
		}
	default:
		order = `p.id DESC`
	}

	limit, offset := b.arg(q.PerPage), b.arg((q.Page-1)*q.PerPage)

	rows, err := s.db.QueryContext(ctx, `SELECT p.id, p.slug, p.title, p.price_cents, p.stock, s.slug, s.name,
		s.ship_from_uf, p.created_at, p.rating_avg_x100, p.rating_count, `+firstImageSQL+from+` ORDER BY `+order+` LIMIT `+limit+` OFFSET `+offset, b.args...)
	if err != nil {
		return Page[ProductCard]{}, fmt.Errorf("search products: %w", err)
	}
	defer rows.Close()

	items, err := scanCards(rows)
	if err != nil {
		return Page[ProductCard]{}, err
	}

	last := int((total + int64(q.PerPage) - 1) / int64(q.PerPage))

	return Page[ProductCard]{Items: items, Total: total, Page: q.Page, PerPage: q.PerPage, LastPage: max(last, 1)}, nil
}

func scanCards(rows *sql.Rows) ([]ProductCard, error) {
	items := []ProductCard{}

	for rows.Next() {
		var (
			c     ProductCard
			img   sql.Null[string]
			width sql.Null[int64]
		)

		if err := rows.Scan(&c.ID, &c.Slug, &c.Title, &c.PriceCents, &c.Stock, &c.ShopSlug, &c.ShopName,
			&c.ShipFromUF, &c.CreatedAt, &c.RatingAvgX100, &c.RatingCount, &img, &width); err != nil {
			return nil, fmt.Errorf("scan product card: %w", err)
		}

		if img.Valid {
			c.ImageURL = imageURL(img.V, width.V > 0, 640)
		}

		items = append(items, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan product cards: %w", err)
	}

	return items, nil
}

// ProductCards returns the visible products among ids in the order of ids
// (recently viewed lists); hidden ids are skipped.
func (s *Service) ProductCards(ctx context.Context, ids []int64) ([]ProductCard, error) {
	if len(ids) == 0 {
		return []ProductCard{}, nil
	}

	b := newSQL(s.db.Dialect())
	in := b.list(ids)

	rows, err := s.db.QueryContext(ctx, `SELECT p.id, p.slug, p.title, p.price_cents, p.stock, s.slug, s.name,
		s.ship_from_uf, p.created_at, p.rating_avg_x100, p.rating_count, `+firstImageSQL+` FROM products p JOIN shops s ON s.id = p.shop_id
		WHERE `+visibleSQL+` AND p.id IN (`+in+`)`, b.args...)
	if err != nil {
		return nil, fmt.Errorf("load product cards: %w", err)
	}
	defer rows.Close()

	cards, err := scanCards(rows)
	if err != nil {
		return nil, err
	}

	byID := make(map[int64]ProductCard, len(cards))
	for i := range cards {
		byID[cards[i].ID] = cards[i]
	}

	out := make([]ProductCard, 0, len(cards))

	for _, id := range ids {
		if c, ok := byID[id]; ok {
			out = append(out, c)
		}
	}

	return out, nil
}

// escapeLike escapes LIKE wildcards in user input (ESCAPE '\').
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// sqlBuilder collects bound arguments and returns their placeholders, so
// raw SQL here never contains input.
type sqlBuilder struct {
	d    database.Dialect
	args []any
}

func newSQL(d database.Dialect) *sqlBuilder { return &sqlBuilder{d: d} }

func (b *sqlBuilder) arg(v any) string {
	b.args = append(b.args, v)

	return b.d.Placeholder(len(b.args))
}

func (b *sqlBuilder) list(ids []int64) string {
	ph := make([]string, len(ids))
	for i, id := range ids {
		ph[i] = b.arg(id)
	}

	return strings.Join(ph, ", ")
}
