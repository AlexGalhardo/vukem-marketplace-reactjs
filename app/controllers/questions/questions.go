// Package questions serves product questions: asking on the product page,
// the seller inbox (/seller/questions) and admin moderation
// (/admin/questions). The qna service scopes every read and write.
package questions

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/auth"
	"github.com/AlexGalhardo/vukem/errs"
	"github.com/AlexGalhardo/vukem/i18n"
	"github.com/AlexGalhardo/vukem/inertia"
	"github.com/AlexGalhardo/vukem/queue"
	"github.com/AlexGalhardo/vukem/web"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/accounts"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/catalog"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/flags"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/mail"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/models"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/qna"
)

// Handlers serves the question pages.
type Handlers struct {
	svc *qna.Service
	acc *accounts.Accounts
}

// FromApp builds the service (notifications through notify, the owner's
// email through app.MailLater) and the handlers.
func FromApp(app *vukem.App, acc *accounts.Accounts, notify qna.Notifier) (*Handlers, error) {
	db, err := app.DB()
	if err != nil {
		return nil, err
	}

	base := strings.TrimSuffix(app.Config.URL, "/")

	// shortcut: the email goes out in the asker's locale (users have no
	// stored locale yet); render it in the owner's once one exists.
	mailOwner := func(ctx context.Context, m qna.NewQuestionMail) error {
		return app.MailLater(ctx, mail.NewQuestion{
			To: m.Owner.Email, OwnerName: qna.FirstName(m.Owner.Name), ProductTitle: m.ProductTitle,
			Question: m.Question, Link: base + "/seller/questions",
			Subject: i18n.T(ctx, "qna.mail.subject", "product", m.ProductTitle),
		}, queue.Retries(3), queue.Backoff(queue.Exponential(10*time.Second)))
	}

	svc, err := qna.New(qna.Config{DB: db, Notify: notify, MailOwner: mailOwner, Log: app.Log, Now: app.Now})
	if err != nil {
		return nil, err
	}

	return &Handlers{svc: svc, acc: acc}, nil
}

// Service returns the qna service.
func (h *Handlers) Service() *qna.Service { return h.svc }

// AskThrottleKey keys the ask rate limit by signed-in user ("" skips it;
// the route requires a session anyway).
func AskThrottleKey(r *http.Request) string {
	u, err := auth.UserFrom[models.User](r.Context(), "web")
	if err != nil {
		return ""
	}

	return "qna-ask:" + u.AuthID()
}

// Localized translates errors whose messages are "qna.*" catalog keys
// into the request locale.
func Localized(fn func(*web.Ctx) error) func(*web.Ctx) error {
	return func(c *web.Ctx) error { return localize(c.Context(), fn(c)) }
}

func localize(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}

	if verr, ok := errors.AsType[*errs.ValidationError](err); ok {
		out := errs.NewValidation()

		for field, msgs := range verr.Fields {
			for _, m := range msgs {
				_ = out.Add(field, translate(ctx, m))
			}
		}

		return out
	}

	if he, ok := errors.AsType[*errs.HTTPError](err); ok && strings.HasPrefix(he.Message, "qna.") {
		// A copy: the sentinel errors are shared package variables.
		return &errs.HTTPError{Status: he.Status, Message: i18n.T(ctx, he.Message), Err: he.Err, Headers: he.Headers}
	}

	return err
}

func translate(ctx context.Context, msg string) string {
	if strings.HasPrefix(msg, "qna.") {
		return i18n.T(ctx, msg)
	}

	return msg
}

func (h *Handlers) user(c *web.Ctx) (models.User, error) {
	u, err := h.acc.Web.User(c)
	if err != nil {
		return models.User{}, errs.ErrUnauthorized
	}

	return u, nil
}

func (h *Handlers) asking(ctx context.Context) bool {
	return h.acc.Flags.Enabled(ctx, flags.ProductQuestions)
}

func questionID(c *web.Ctx) (int64, error) {
	id, err := c.ParamInt("id")
	if err != nil || id <= 0 {
		return 0, qna.ErrNotFound
	}

	return id, nil
}

// ProductProps are the product page's question props: the page of
// questions (?q_page=) and whether the viewer may ask.
func (h *Handlers) ProductProps(c *web.Ctx, p catalog.ProductDetail) (inertia.Props, error) {
	ctx := c.Context()

	page, err := h.svc.ProductQuestions(ctx, p.ID, c.QueryInt("q_page", 1))
	if err != nil {
		return nil, err
	}

	own := false
	if u, err := h.acc.Web.User(c); err == nil {
		own, err = h.svc.OwnsShop(ctx, u.ID, p.Shop.ID)
		if err != nil {
			return nil, err
		}
	}

	return inertia.Props{
		"questions": page,
		"qna":       map[string]any{"enabled": h.asking(ctx), "own_product": own},
	}, nil
}

// Ask stores a question on /p/{slug} (POST /p/{slug}/questions).
func (h *Handlers) Ask(c *web.Ctx) error {
	if !h.asking(c.Context()) {
		return qna.ErrNotFound
	}

	u, err := h.user(c)
	if err != nil {
		return err
	}

	in, err := web.Parse(c, qna.AskSchema)
	if err != nil {
		return err
	}

	slug := c.Param("slug")
	if _, err := h.svc.Ask(c.Context(), u.ID, slug, in.Body); err != nil {
		return err
	}

	inertia.Flash(c, "success", i18n.T(c.Context(), "qna.flash.asked"))

	return c.Redirect("/p/" + slug)
}

// SellerIndex renders the seller inbox: unanswered questions by default,
// answered ones with ?filter=answered.
func (h *Handlers) SellerIndex(c *web.Ctx) error {
	u, err := h.user(c)
	if err != nil {
		return err
	}

	filter := qna.FilterUnanswered
	if c.Query("filter") == qna.FilterAnswered {
		filter = qna.FilterAnswered
	}

	page, err := h.svc.ShopQuestions(c.Context(), u.ID, filter, c.QueryInt("page", 1))
	if errors.Is(err, qna.ErrNoShop) {
		return c.Redirect("/sell")
	}

	if err != nil {
		return err
	}

	unanswered, err := h.svc.UnansweredCount(c.Context(), u.ID)
	if err != nil {
		return err
	}

	return inertia.Render(c, "Seller/Questions", inertia.Props{
		"filter": filter, "questions": page, "unanswered": unanswered,
	})
}

// SellerAnswer answers a question of the seller's shop
// (POST /seller/questions/{id}/answer); other shops' ids are 404.
func (h *Handlers) SellerAnswer(c *web.Ctx) error {
	u, err := h.user(c)
	if err != nil {
		return err
	}

	id, err := questionID(c)
	if err != nil {
		return err
	}

	in, err := web.Parse(c, qna.AnswerSchema)
	if err != nil {
		return err
	}

	if err := h.svc.Answer(c.Context(), u.ID, id, in.Answer); err != nil {
		return err
	}

	inertia.Flash(c, "success", i18n.T(c.Context(), "qna.flash.answered"))

	return c.Redirect("/seller/questions")
}

// AdminIndex renders the moderation list (?filter=hidden for hidden only).
func (h *Handlers) AdminIndex(c *web.Ctx) error {
	filter := qna.FilterAll
	if c.Query("filter") == qna.FilterHidden {
		filter = qna.FilterHidden
	}

	page, err := h.svc.AdminQuestions(c.Context(), filter, c.QueryInt("page", 1))
	if err != nil {
		return err
	}

	return inertia.Render(c, "Admin/Questions", inertia.Props{"filter": filter, "questions": page})
}

// AdminHide hides a question with a reason (POST /admin/questions/{id}/hide).
func (h *Handlers) AdminHide(c *web.Ctx) error {
	u, err := h.user(c)
	if err != nil {
		return err
	}

	id, err := questionID(c)
	if err != nil {
		return err
	}

	in, err := web.Parse(c, qna.HideSchema)
	if err != nil {
		return err
	}

	if err := h.svc.Hide(c.Context(), u.ID, id, in.Reason); err != nil {
		return err
	}

	web.AuditEvent(c.Request(), "admin.question_hidden", "question_id", id, "actor_id", u.AuthID())
	inertia.Flash(c, "success", i18n.T(c.Context(), "qna.flash.hidden"))

	return c.Redirect(adminBack(c))
}

// AdminUnhide restores a hidden question (POST /admin/questions/{id}/unhide).
func (h *Handlers) AdminUnhide(c *web.Ctx) error {
	u, err := h.user(c)
	if err != nil {
		return err
	}

	id, err := questionID(c)
	if err != nil {
		return err
	}

	if err := h.svc.Unhide(c.Context(), id); err != nil {
		return err
	}

	web.AuditEvent(c.Request(), "admin.question_unhidden", "question_id", id, "actor_id", u.AuthID())
	inertia.Flash(c, "success", i18n.T(c.Context(), "qna.flash.unhidden"))

	return c.Redirect(adminBack(c))
}

// adminBack is the moderation page with the filter the form came from
// (a fixed set: never an open redirect).
func adminBack(c *web.Ctx) string {
	if c.Query("filter") == qna.FilterHidden {
		return "/admin/questions?filter=" + qna.FilterHidden
	}

	return "/admin/questions"
}
