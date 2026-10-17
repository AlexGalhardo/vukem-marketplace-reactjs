// Package storefront serves the public catalog pages (home, search,
// category, product, shop) and the product images.
package storefront

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/AlexGalhardo/vukem/i18n"
	"github.com/AlexGalhardo/vukem/inertia"
	"github.com/AlexGalhardo/vukem/storage"
	"github.com/AlexGalhardo/vukem/web"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/accounts"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/catalog"
)

// Session keys.
const (
	recentKey = "catalog.recent"
	anonKey   = "catalog.anon"
	// maxRecent is how many recently viewed products are remembered.
	maxRecent = 12
)

// ReviewsBlock builds the reviews section of a product page for the
// current viewer (controllers/reviews.Handlers.ProductBlock).
type ReviewsBlock func(c *web.Ctx, productID, shopID int64) (any, error)

// Handlers serves the storefront.
type Handlers struct {
	svc     *catalog.Service
	acc     *accounts.Accounts
	reviews ReviewsBlock
}

// New returns the handlers.
func New(svc *catalog.Service, acc *accounts.Accounts) *Handlers {
	return &Handlers{svc: svc, acc: acc}
}

// WithReviews adds the reviews section to the product page.
func (h *Handlers) WithReviews(fn ReviewsBlock) *Handlers {
	h.reviews = fn
	return h
}

// userID is the signed-in user's id, or 0 for guests.
func (h *Handlers) userID(c *web.Ctx) int64 {
	u, err := h.acc.Web.User(c)
	if err != nil {
		return 0
	}

	return u.ID
}

// visitor returns the user id, or a random per-session id for guests, for
// the view/search history (no IP, no session id).
func (h *Handlers) visitor(c *web.Ctx) (int64, string) {
	if id := h.userID(c); id != 0 {
		return id, ""
	}

	anon, _ := c.Session().Get(anonKey).(string)
	if anon == "" {
		anon = catalog.NewAnonID()
		c.Session().Put(anonKey, anon)
	}

	return 0, anon
}

// recent returns the recently viewed product ids, newest first.
func recent(c *web.Ctx) []int64 {
	raw, _ := c.Session().Get(recentKey).(string)

	var ids []int64

	for part := range strings.SplitSeq(raw, ",") {
		if id, err := strconv.ParseInt(part, 10, 64); err == nil && id > 0 {
			ids = append(ids, id)
		}
	}

	return ids
}

// remember puts id first in the recently viewed list. Stored as a string:
// session values round-trip through JSON.
func remember(c *web.Ctx, id int64) {
	ids := []string{strconv.FormatInt(id, 10)}

	for _, old := range recent(c) {
		if old != id && len(ids) < maxRecent {
			ids = append(ids, strconv.FormatInt(old, 10))
		}
	}

	c.Session().Put(recentKey, strings.Join(ids, ","))
}

// Home renders the home page sections.
func (h *Handlers) Home(c *web.Ctx) error {
	ctx := c.Context()
	locale := i18n.Locale(ctx)

	cats, err := h.svc.Categories(ctx)
	if err != nil {
		return err
	}

	latest, err := h.svc.Search(ctx, catalog.Query{Sort: catalog.SortNewest, PerPage: 12}, true)
	if err != nil {
		return err
	}

	deals, err := h.svc.Search(ctx, catalog.Query{Sort: catalog.SortPriceAsc, PerPage: 6}, true)
	if err != nil {
		return err
	}

	viewed, err := h.svc.ProductCards(ctx, recent(c))
	if err != nil {
		return err
	}

	return inertia.Render(c, "Home", inertia.Props{
		"categories": catalog.LocalizedCategories(topLevel(cats), locale),
		"latest":     latest.Items,
		"deals":      deals.Items,
		"recent":     viewed,
	})
}

func topLevel(all []catalog.CategoryView) []catalog.CategoryView {
	out := make([]catalog.CategoryView, 0, len(all))

	for _, c := range all {
		if c.Parent == "" {
			out = append(out, c)
		}
	}

	return out
}

// Search renders /search with the filters from the query string.
func (h *Handlers) Search(c *web.Ctx) error {
	q, err := web.ParseQuery(c, catalog.QuerySchema)
	if err != nil {
		return err
	}

	return h.results(c, q, nil)
}

// Category renders /c/{slug}: the search page scoped to one category.
func (h *Handlers) Category(c *web.Ctx) error {
	cat, err := h.svc.CategoryBySlug(c.Context(), c.Param("slug"))
	if err != nil {
		return err
	}

	q, err := web.ParseQuery(c, catalog.QuerySchema)
	if err != nil {
		return err
	}

	q.Category = cat.Slug
	ref := cat.Ref(i18n.Locale(c.Context()))

	return h.results(c, q, &ref)
}

func (h *Handlers) results(c *web.Ctx, q catalog.Query, category *catalog.CategoryRef) error {
	ctx := c.Context()
	uid := h.userID(c)

	page, err := h.svc.Search(ctx, q, uid == 0)
	if err != nil {
		return err
	}

	if !inertia.IsPrefetch(c.Request()) {
		uid, anon := h.visitor(c)
		h.svc.RecordSearch(ctx, q, page.Total, uid, anon)
	}

	cats, err := h.svc.Categories(ctx)
	if err != nil {
		return err
	}

	return inertia.Render(c, "Catalog/Search", inertia.Props{
		"query":      q.Normalized(),
		"results":    page,
		"category":   category,
		"categories": catalog.LocalizedCategories(cats, i18n.Locale(ctx)),
		"ufs":        catalog.UFs,
	})
}

// Product renders /p/{slug} and records the view.
func (h *Handlers) Product(c *web.Ctx) error {
	ctx := c.Context()

	p, err := h.svc.PublicProduct(ctx, c.Param("slug"), i18n.Locale(ctx))
	if err != nil {
		return err
	}

	more, err := h.svc.Search(ctx, catalog.Query{Shop: p.Shop.Slug, Sort: catalog.SortNewest, PerPage: 7}, true)
	if err != nil {
		return err
	}

	related := make([]catalog.ProductCard, 0, len(more.Items))
	for i := range more.Items {
		if more.Items[i].ID != p.ID && len(related) < 6 {
			related = append(related, more.Items[i])
		}
	}

	// Partial reloads (sorting or paging the reviews) are not new views.
	if !inertia.IsPrefetch(c.Request()) && c.Request().Header.Get("X-Inertia-Partial-Data") == "" {
		uid, anon := h.visitor(c)
		h.svc.RecordView(ctx, p, uid, anon)
		remember(c, p.ID)
	}

	props := inertia.Props{"product": p, "more": related}

	if h.reviews != nil {
		block, err := h.reviews(c, p.ID, p.Shop.ID)
		if err != nil {
			return err
		}

		props["reviews"] = block
	}

	return inertia.Render(c, "Catalog/Product", props)
}

// Shop renders the storefront /s/{slug}.
func (h *Handlers) Shop(c *web.Ctx) error {
	ctx := c.Context()

	shop, err := h.svc.ShopBySlug(ctx, c.Param("slug"))
	if err != nil {
		return err
	}

	page, err := h.svc.Search(
		ctx,
		catalog.Query{Shop: shop.Slug, Sort: catalog.SortNewest, Page: c.QueryInt("page", 1)},
		true,
	)
	if err != nil {
		return err
	}

	return inertia.Render(c, "Catalog/Shop", inertia.Props{"shop": shop, "results": page})
}

// Media streams a stored image (variant, or original until processed).
// Only stored image names reach the disk (no arbitrary object reads);
// names are random and never rewritten, so responses cache for a year.
func (h *Handlers) Media(c *web.Ctx) error {
	p := c.Param("kind") + "/" + c.Param("id") + "/" + c.Param("file")

	ctype, ok := catalog.MediaObject(p)
	if !ok {
		return catalog.ErrNotFound
	}

	rc, err := h.svc.Disk().Open(c.Context(), p)
	if errors.Is(err, storage.ErrNotFound) {
		return catalog.ErrNotFound
	}

	if err != nil {
		return err
	}
	defer rc.Close()

	w := c.Writer()
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.WriteHeader(http.StatusOK)

	_, err = io.Copy(w, rc)

	return err
}
