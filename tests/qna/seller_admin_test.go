package qna_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/AlexGalhardo/vukem/events"
	"github.com/AlexGalhardo/vukem/orm"
	"github.com/AlexGalhardo/vukem/security"
	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/qna"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/realtime"
)

type published struct {
	userID  int64
	channel string
	payload map[string]any
}

type fakeNotifier struct {
	mu   sync.Mutex
	sent []published
}

func (f *fakeNotifier) Publish(_ context.Context, userID int64, channel string, payload any) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	m, _ := payload.(map[string]any)
	f.sent = append(f.sent, published{userID, channel, m})

	return nil
}

func TestServiceNotifiesOwnerAndAsker(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	asker := e.user(t, "Lia")
	notify := &fakeNotifier{}

	var mails []qna.NewQuestionMail

	svc, err := qna.New(
		qna.Config{DB: db(t, e.app), Notify: notify, MailOwner: func(_ context.Context, m qna.NewQuestionMail) error {
			mails = append(mails, m)
			return nil
		}},
	)
	if err != nil {
		t.Fatal(err)
	}

	q, err := svc.Ask(t.Context(), asker.ID, e.product.Slug, "Is the jar made of glass?")
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.Answer(t.Context(), e.owner.ID, q.ID, "Yes, tempered glass."); err != nil {
		t.Fatal(err)
	}

	want := []published{
		{e.owner.ID, realtime.Notifications, map[string]any{
			"type": "qna.question", "question_id": q.ID, "product": e.product.Slug, "title": e.product.Title,
		}},
		{asker.ID, realtime.Notifications, map[string]any{"type": "qna.answer", "question_id": q.ID}},
	}
	if diff := cmp.Diff(want, notify.sent, cmp.AllowUnexported(published{})); diff != "" {
		t.Fatalf("published (-want +got):\n%s", diff)
	}

	if len(mails) != 1 || mails[0].Owner.ID != e.owner.ID || mails[0].Question != "Is the jar made of glass?" {
		t.Fatalf("owner mails = %+v, want one for the owner", mails)
	}
}

func TestSellerInboxAndAnswer(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	asker := e.user(t, "Joana Dark")
	old := e.question(t, asker, "Old question, still open?", time.Now().Add(-time.Hour))
	q := e.question(t, asker, "Does it have a warranty?", time.Now())

	seller := e.inertia(t, e.owner)
	seller.Get("/seller/questions").AssertOK().AssertInertia("Seller/Questions").
		PropEquals("filter", "unanswered").PropEquals("unanswered", float64(2)).
		HasProp("questions.data", 2).
		PropEquals("questions.data.0.id", float64(q.ID)).
		PropEquals("questions.data.0.product.slug", e.product.Slug).
		PropEquals("questions.data.0.asker_name", "Joana")

	answer := "/seller/questions/" + id(q.ID) + "/answer"
	seller.PostJSON(answer, map[string]any{"answer": "Yes, 12 months."}).AssertRedirect("/seller/questions")

	got := e.load(t, q.ID)
	if got.Answer.V != "Yes, 12 months." || !got.AnsweredAt.Valid {
		t.Fatalf("answered question = %+v", got)
	}

	seller.Get("/seller/questions").AssertInertia("Seller/Questions").
		PropEquals("unanswered", float64(1)).PropEquals("questions.data.0.id", float64(old.ID))
	seller.Get("/seller/questions?filter=answered").AssertInertia("Seller/Questions").
		PropEquals("filter", "answered").HasProp("questions.data", 1).
		PropEquals("questions.data.0.answer", "Yes, 12 months.")

	// The answer is public.
	e.app.Client(t).Inertia("").Get("/p/"+e.product.Slug).AssertInertia("Catalog/Product").
		PropEquals("questions.data.0.answer", "Yes, 12 months.")

	// Users without a shop go open one.
	e.inertia(t, asker).Get("/seller/questions").AssertRedirect("/sell")
}

func TestAnswerValidation(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	q := e.question(t, e.user(t, "Asker"), "What is the voltage?", time.Now())
	path := "/seller/questions/" + id(q.ID) + "/answer"
	c := e.json(t, e.owner)

	for _, bad := range []string{"", "   ", strings.Repeat("a", 1001), "Call me at (11) 98765-4321", "See minhaloja.com.br"} {
		c.PostJSON(path, map[string]any{"answer": bad}).AssertUnprocessable().AssertJSONValidationErrors("answer")
	}

	if e.load(t, q.ID).Answer.Valid {
		t.Fatal("an invalid answer was stored")
	}

	c.PostJSON(path, map[string]any{"answer": "y"}).AssertRedirect("/seller/questions")
	c.PostJSON(path, map[string]any{"answer": strings.Repeat("z", 1000)}).AssertRedirect("/seller/questions")

	if got := e.load(t, q.ID).Answer.V; got != strings.Repeat("z", 1000) {
		t.Fatalf("answer = %d chars, want the 1000-char update", len(got))
	}
}

func TestAnswerIDOR(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	q := e.question(t, e.user(t, "Asker"), "Is it available in blue?", time.Now())
	path := "/seller/questions/" + id(q.ID) + "/answer"
	body := map[string]any{"answer": "Hijacked answer"}

	other := e.user(t, "Rival Seller")
	e.sellerWithProduct(t, other, "Loja Rival", "Produto Rival")
	e.json(t, other).PostJSON(path, body).AssertNotFound()

	// A user without a shop, an unknown id and a malformed id are 404 too.
	e.json(t, e.user(t, "No Shop")).PostJSON(path, body).AssertNotFound()
	e.json(t, e.owner).PostJSON("/seller/questions/999999/answer", body).AssertNotFound()
	e.json(t, e.owner).PostJSON("/seller/questions/abc/answer", body).AssertNotFound()

	// The rival's inbox does not list it.
	e.inertia(t, other).Get("/seller/questions").AssertInertia("Seller/Questions").HasProp("questions.data", 0)

	if e.load(t, q.ID).Answer.Valid {
		t.Fatal("another seller answered the question")
	}
}

func TestAdminHidesAndUnhides(t *testing.T) {
	t.Parallel()

	e := newEnv(t)

	var (
		mu     sync.Mutex
		audits []security.Audit
	)

	events.Listen(e.app.Events, func(_ context.Context, a security.Audit) error {
		mu.Lock()
		defer mu.Unlock()

		audits = append(audits, a)

		return nil
	})

	asker := e.user(t, "Troll")
	q := e.question(t, asker, "Rude question about the seller", time.Now())
	keep := e.question(t, asker, "A fair question about size", time.Now().Add(-time.Minute))
	admin := e.admin(t)
	c := e.json(t, admin)

	e.inertia(t, admin).Get("/admin/questions").AssertOK().AssertInertia("Admin/Questions").HasProp("questions.data", 2)

	// Non-admins never see the moderation routes.
	e.json(t, e.owner).PostJSON("/admin/questions/"+id(q.ID)+"/hide", map[string]any{"reason": "spam"}).AssertNotFound()
	e.inertia(t, asker).Get("/admin/questions").AssertNotFound()

	c.PostJSON("/admin/questions/"+id(q.ID)+"/hide", map[string]any{"reason": ""}).AssertUnprocessable()
	c.PostJSON("/admin/questions/999999/hide", map[string]any{"reason": "Offensive"}).AssertNotFound()
	c.PostJSON("/admin/questions/"+id(q.ID)+"/hide", map[string]any{"reason": "Offensive language"}).
		AssertRedirect("/admin/questions")

	got := e.load(t, q.ID)
	if !got.HiddenAt.Valid || got.HiddenBy != (sql.Null[int64]{V: admin.ID, Valid: true}) ||
		got.HiddenReason.V != "Offensive language" {
		t.Fatalf("hidden question = %+v", got)
	}

	// Hidden: gone from the product page and the seller inbox, kept for admins.
	e.app.Client(t).Inertia("").Get("/p/"+e.product.Slug).AssertInertia("Catalog/Product").
		PropEquals("questions.total", float64(1)).PropEquals("questions.data.0.id", float64(keep.ID))
	e.inertia(t, e.owner).Get("/seller/questions").AssertInertia("Seller/Questions").
		PropEquals("unanswered", float64(1)).HasProp("questions.data", 1)
	e.json(t, e.owner).
		PostJSON("/seller/questions/"+id(q.ID)+"/answer", map[string]any{"answer": "No"}).
		AssertNotFound()
	e.inertia(t, admin).Get("/admin/questions?filter=hidden").AssertInertia("Admin/Questions").
		HasProp("questions.data", 1).PropEquals("questions.data.0.hidden_reason", "Offensive language")

	c.PostJSON("/admin/questions/"+id(q.ID)+"/unhide?filter=hidden", nil).
		AssertRedirect("/admin/questions?filter=hidden")

	if e.load(t, q.ID).HiddenAt.Valid {
		t.Fatal("unhide left hidden_at set")
	}

	e.app.Client(t).Inertia("").Get("/p/"+e.product.Slug).AssertInertia("Catalog/Product").
		PropEquals("questions.total", float64(2))

	mu.Lock()
	defer mu.Unlock()

	names := make([]string, 0, len(audits))
	for _, a := range audits {
		names = append(names, a.Name)

		if a.Attrs["question_id"] != q.ID || a.Attrs["actor_id"] != admin.AuthID() {
			t.Fatalf("audit attrs = %v, want question %d and actor %d", a.Attrs, q.ID, admin.ID)
		}
	}

	if diff := cmp.Diff([]string{"admin.question_hidden", "admin.question_unhidden"}, names); diff != "" {
		t.Fatalf("audit events (-want +got):\n%s", diff)
	}
}

func TestInsertDemo(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	d := db(t, e.app)
	asker := e.user(t, "Demo Buyer")
	at := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)

	answered := qna.Question{
		CreatedAt: at.Add(-time.Hour),
		ProductID: e.product.ID, AskerID: asker.ID, Body: "  Does it crush ice?  ",
		Answer: sql.Null[string]{V: "Yes.", Valid: true}, AnsweredAt: orm.NullTime{V: at, Valid: true},
	}

	open := qna.Question{ProductID: e.product.ID, AskerID: asker.ID, Body: "How loud is the motor?"}

	if err := qna.InsertDemo(t.Context(), d, []qna.Question{answered, open}); err != nil {
		t.Fatal(err)
	}

	vukemtest.AssertDatabaseHas(t, e.app, "product_questions", map[string]any{
		"body": "Does it crush ice?", "shop_id": e.shop.ID, "answer": "Yes.",
	})
	vukemtest.AssertDatabaseCount(t, e.app, "product_questions", 2)

	bad := []qna.Question{
		{ProductID: e.product.ID, AskerID: e.owner.ID, Body: "The owner asking here?"},
		{ProductID: e.product.ID, ShopID: e.shop.ID + 99, AskerID: asker.ID, Body: "Wrong shop for this one"},
		{ProductID: 999999, AskerID: asker.ID, Body: "Unknown product here?"},
		{ProductID: e.product.ID, AskerID: asker.ID, Body: "Call me on (11) 98765-4321"},
		{ProductID: e.product.ID, AskerID: asker.ID, Body: "short"},
		{
			ProductID: e.product.ID,
			AskerID:   asker.ID,
			Body:      "Answer without a date?",
			Answer:    sql.Null[string]{V: "Yes", Valid: true},
		},
	}
	for i, row := range bad {
		if err := qna.InsertDemo(t.Context(), d, []qna.Question{row}); err == nil {
			t.Fatalf("bad row %d inserted", i)
		}
	}

	vukemtest.AssertDatabaseCount(t, e.app, "product_questions", 2)

	if err := qna.InsertDemo(t.Context(), d, nil); err != nil {
		t.Fatal(err)
	}

	_, err := qna.New(qna.Config{})
	if err == nil || errors.Is(err, qna.ErrNotFound) {
		t.Fatalf("New without a DB = %v, want a config error", err)
	}
}
