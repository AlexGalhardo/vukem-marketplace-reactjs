// Package reviews serves the review form, helpful votes, the shop's reply,
// admin moderation and the reviews block of the product page (Inertia
// pages resources/js/pages/Reviews and Admin/Reviews).
package reviews

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/AlexGalhardo/vukem/errs"
	"github.com/AlexGalhardo/vukem/i18n"
	"github.com/AlexGalhardo/vukem/inertia"
	"github.com/AlexGalhardo/vukem/web"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/accounts"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/flags"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/models"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/reviews"
)

// Handlers serves the review pages and actions.
type Handlers struct {
	Reviews  *reviews.Service
	Accounts *accounts.Accounts
}

// Block is the reviews section of a product page.
type Block struct {
	// Enabled mirrors the reviews flag: off, the page offers no form,
	// votes or replies.
	Enabled bool            `json:"enabled"`
	Summary reviews.Summary `json:"summary"`
	List    reviews.Page    `json:"list"`
	// PendingItem is the viewer's order line of this product waiting for
	// a review (0: none); the page links to its form.
	PendingItem int64 `json:"pending_item"`
	// CanVote: signed in. CanReply: the viewer owns the product's shop.
	CanVote  bool `json:"can_vote"`
	CanReply bool `json:"can_reply"`
	MaxReply int  `json:"max_reply"`
}

func (h *Handlers) viewer(c *web.Ctx) (models.User, bool) {
	u, err := h.Accounts.Web.User(c)
	return u, err == nil
}

func (h *Handlers) enabled(c *web.Ctx) bool {
	return h.Accounts.Flags.Enabled(c.Context(), flags.Reviews)
}

// ProductBlock builds the reviews section of product productID, of shop
// shopID, for the current viewer. The list follows the
// "rsort" (newest, helpful) and "rpage" query parameters.
func (h *Handlers) ProductBlock(c *web.Ctx, productID, shopID int64) (any, error) {
	ctx := c.Context()
	u, signedIn := h.viewer(c)

	b := Block{Enabled: h.enabled(c), MaxReply: reviews.MaxReply}

	var err error
	if b.Summary, err = h.Reviews.Summary(ctx, productID); err != nil {
		return Block{}, err
	}

	if b.List, err = h.Reviews.List(ctx, productID, u.ID, c.Query("rsort"), c.QueryInt("rpage", 1)); err != nil {
		return Block{}, err
	}

	if !b.Enabled || !signedIn {
		return b, nil
	}

	b.CanVote = true
	if b.CanReply, err = h.Reviews.OwnsShop(ctx, u.ID, shopID); err != nil {
		return Block{}, err
	}

	if b.PendingItem, err = h.Reviews.PendingItem(ctx, u.ID, productID); err != nil {
		return Block{}, err
	}

	return b, nil
}

func paramID(c *web.Ctx, name string) (int64, error) {
	id, err := c.ParamInt(name)
	if err != nil || id <= 0 {
		return 0, errs.ErrNotFound
	}

	return id, nil
}

// notFound turns reviews.ErrNotFound into the app's 404.
func notFound(err error) error {
	if errors.Is(err, reviews.ErrNotFound) {
		return errs.ErrNotFound
	}

	return err
}

func productURL(slug string) string { return "/p/" + slug }

// ShowForm renders the review form of order line {item}. Lines the user
// cannot review are 404; a line already reviewed goes back to its product.
func (h *Handlers) ShowForm(c *web.Ctx) error {
	u, ok := h.viewer(c)
	if !ok {
		return errs.ErrUnauthorized
	}

	id, err := paramID(c, "item")
	if err != nil {
		return err
	}

	it, err := h.Reviews.Eligible(c.Context(), u.ID, id)
	if err != nil {
		return notFound(err)
	}

	if it.ReviewID != 0 {
		inertia.Flash(c, "status", i18n.T(c.Context(), "reviews.errors.already_reviewed"))
		return c.Redirect(productURL(it.ProductSlug))
	}

	return inertia.Render(c, "Reviews/Form", inertia.Props{
		"item":   it,
		"limits": map[string]int{"title": reviews.MaxTitle, "body": reviews.MaxBody},
	})
}

// Store publishes the review of order line {item}.
func (h *Handlers) Store(c *web.Ctx) error {
	u, ok := h.viewer(c)
	if !ok {
		return errs.ErrUnauthorized
	}

	id, err := paramID(c, "item")
	if err != nil {
		return err
	}

	// Eligibility first: an ineligible line is 404 whatever the body says.
	if _, err := h.Reviews.Eligible(c.Context(), u.ID, id); err != nil {
		return notFound(err)
	}

	in, err := web.Parse(c, reviews.InputSchema)
	if err != nil {
		return err
	}

	it, err := h.Reviews.Create(c.Context(), u.ID, id, in)
	if errors.Is(err, reviews.ErrAlreadyReviewed) {
		inertia.Flash(c, "status", i18n.T(c.Context(), "reviews.errors.already_reviewed"))
		return c.Redirect(productURL(it.ProductSlug))
	}

	if err != nil {
		return notFound(err)
	}

	inertia.Flash(c, "success", i18n.T(c.Context(), "reviews.flash.created"))

	return c.Redirect(productURL(it.ProductSlug))
}

// Vote marks review {id} helpful for the signed-in user (once; not their
// own review).
func (h *Handlers) Vote(c *web.Ctx) error {
	u, ok := h.viewer(c)
	if !ok {
		return errs.ErrUnauthorized
	}

	id, err := paramID(c, "id")
	if err != nil {
		return err
	}

	added, err := h.Reviews.Vote(c.Context(), u.ID, id)
	if errors.Is(err, reviews.ErrOwnReview) {
		return errs.New(http.StatusForbidden, i18n.T(c.Context(), "reviews.errors.own_review"))
	}

	if err != nil {
		return notFound(err)
	}

	if added {
		inertia.Flash(c, "success", i18n.T(c.Context(), "reviews.flash.voted"))
	}

	return c.Back("/")
}

// Reply publishes the shop's answer to review {id}: the owner of the
// reviewed product's shop only, once.
func (h *Handlers) Reply(c *web.Ctx) error {
	u, ok := h.viewer(c)
	if !ok {
		return errs.ErrUnauthorized
	}

	id, err := paramID(c, "id")
	if err != nil {
		return err
	}

	in, err := web.Parse(c, reviews.ReplySchema)
	if err != nil {
		return err
	}

	err = h.Reviews.Reply(c.Context(), u.ID, id, in.Body)
	if errors.Is(err, reviews.ErrAlreadyReplied) {
		return errs.NewValidation().Add("body", "reviews.errors.already_replied")
	}

	if err != nil {
		return notFound(err)
	}

	inertia.Flash(c, "success", i18n.T(c.Context(), "reviews.flash.replied"))

	return c.Back("/")
}

// AdminIndex lists reviews for moderation ("filter": all or hidden).
func (h *Handlers) AdminIndex(c *web.Ctx) error {
	page, err := h.Reviews.AdminList(c.Context(), c.Query("filter"), c.QueryInt("page", 1))
	if err != nil {
		return err
	}

	return inertia.Render(c, "Admin/Reviews", inertia.Props{
		"reviews": page,
		"limits":  map[string]int{"reasonMin": reviews.MinReason, "reasonMax": reviews.MaxReason},
	})
}

// Hide hides review {id} with a reason; the change is audited.
func (h *Handlers) Hide(c *web.Ctx) error {
	in, err := web.Parse(c, reviews.HideSchema)
	if err != nil {
		return err
	}

	return h.moderate(c, true, in.Reason)
}

// Unhide shows review {id} again; the change is audited.
func (h *Handlers) Unhide(c *web.Ctx) error { return h.moderate(c, false, "") }

func (h *Handlers) moderate(c *web.Ctx, hide bool, reason string) error {
	admin, ok := h.viewer(c)
	if !ok {
		return errs.ErrUnauthorized
	}

	id, err := paramID(c, "id")
	if err != nil {
		return err
	}

	changed, err := h.Reviews.SetHidden(c.Context(), admin.ID, id, hide, reason)
	if err != nil {
		return notFound(err)
	}

	event, flash := "review.shown", "reviews.flash.shown"
	if hide {
		event, flash = "review.hidden", "reviews.flash.hidden"
	}

	if changed {
		web.AuditEvent(c.Request(), event, "review_id", strconv.FormatInt(id, 10), "actor_id", admin.AuthID())
	}

	inertia.Flash(c, "success", i18n.T(c.Context(), flash))

	return c.Back("/admin/reviews")
}
