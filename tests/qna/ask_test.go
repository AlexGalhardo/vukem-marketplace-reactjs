package qna_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/catalog"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/flags"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/internal/testing/flagtest"
)

func TestAskLengthBoundaries(t *testing.T) {
	t.Parallel()

	e := newEnv(t)

	cases := []struct {
		name string
		body string
		ok   bool
	}{
		{"9 chars", strings.Repeat("a", 9), false},
		{"10 chars", strings.Repeat("a", 10), true},
		{"10 chars padded", "   " + strings.Repeat("b", 10) + "  \n", true},
		{"9 chars padded", "     " + strings.Repeat("c", 9) + "     ", false},
		{"10 runes", strings.Repeat("é", 10), true},
		{"500 chars", strings.Repeat("d", 500), true},
		{"501 chars", strings.Repeat("e", 501), false},
	}
	for _, tc := range cases {
		// A fresh asker per case: the rate limit is per user.
		res := e.json(t, e.user(t, "Bia Buyer")).PostJSON(e.askPath(), map[string]any{"body": tc.body})
		if tc.ok {
			res.AssertRedirect("/p/" + e.product.Slug)
			vukemtest.AssertDatabaseHas(
				t,
				e.app,
				"product_questions",
				map[string]any{"body": strings.TrimSpace(tc.body)},
			)

			continue
		}

		res.AssertUnprocessable().AssertJSONValidationErrors("body")
	}

	vukemtest.AssertDatabaseCount(t, e.app, "product_questions", 4)
}

func TestAskRejectsContactData(t *testing.T) {
	t.Parallel()

	e := newEnv(t)

	bodies := []string{
		"Compre direto em https://outraloja.example/oferta",
		"Me chama no www.minhaloja.com, faço mais barato",
		"Mais barato em minhaloja.com.br, confira",
		"Manda um email para ana.souza@example.com",
		"Me segue no insta @loja_da_ana para ofertas",
		"Fala comigo: ana arroba provedor ponto com",
		"Meu whats é (11) 98765-4321, chama lá",
		"Liga no 11 9 8765 4321 que eu explico",
		"Telefone 11.98765.4321 para combinar",
		"Ligue 9 8 7 6 5 4 3 2 1 por favor",
		"Telefone fixo 3333-4444 em horário comercial",
	}
	for _, body := range bodies {
		res := e.json(t, e.user(t, "Caio Curioso")).PostJSON(e.askPath(), map[string]any{"body": body})
		res.AssertUnprocessable().AssertJSONValidationErrors("body")

		if !strings.Contains(res.Body(), "keep the conversation on the platform") {
			t.Fatalf("%q: error body %s, want the friendly no-contact message", body, res.Body())
		}
	}

	vukemtest.AssertDatabaseCount(t, e.app, "product_questions", 0)

	// The message follows the request locale.
	pt := e.json(t, e.user(t, "Pedro")).WithCookie("lang", "pt-BR").
		PostJSON(e.askPath(), map[string]any{"body": "Liga no (21) 99999-8888 agora"})
	pt.AssertUnprocessable()

	if !strings.Contains(pt.Body(), "mantenha a conversa na plataforma") {
		t.Fatalf("pt-BR error body %s, want the Portuguese message", pt.Body())
	}
}

func TestAskRateLimit(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	c := e.json(t, e.user(t, "Rita Rapid"))

	for i := range 5 {
		c.PostJSON(e.askPath(), map[string]any{"body": "Question number " + strings.Repeat("x", i+1) + " here?"}).
			AssertRedirect("/p/" + e.product.Slug)
	}

	c.PostJSON(e.askPath(), map[string]any{"body": "One question too many, please?"}).
		AssertStatus(http.StatusTooManyRequests)
	vukemtest.AssertDatabaseCount(t, e.app, "product_questions", 5)

	// The budget is per user: someone else can still ask.
	e.json(t, e.user(t, "Other Buyer")).PostJSON(e.askPath(), map[string]any{"body": "Is there a warranty included?"}).
		AssertRedirect("/p/" + e.product.Slug)
}

func TestOwnerCannotAskOnOwnProduct(t *testing.T) {
	t.Parallel()

	e := newEnv(t)

	res := e.json(t, e.owner).PostJSON(e.askPath(), map[string]any{"body": "Can I ask about my own product?"})
	res.AssertForbidden()

	if !strings.Contains(res.Body(), "your own shop") {
		t.Fatalf("own product error body %s, want the friendly message", res.Body())
	}

	vukemtest.AssertDatabaseCount(t, e.app, "product_questions", 0)

	// The page tells the owner instead of showing the form.
	e.inertia(t, e.owner).Get("/p/"+e.product.Slug).AssertOK().AssertInertia("Catalog/Product").
		PropEquals("qna.own_product", true)
	e.inertia(t, e.user(t, "Buyer")).Get("/p/"+e.product.Slug).AssertOK().AssertInertia("Catalog/Product").
		PropEquals("qna.own_product", false).PropEquals("qna.enabled", true)
}

func TestAskOnlyOnVisibleProducts(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	body := map[string]any{"body": "Is this still available for sale?"}

	draft := e.productOf(t, e.owner, "Rascunho Secreto", catalog.StatusDraft)
	archived := e.productOf(t, e.owner, "Produto Arquivado", catalog.StatusArchived)

	asker := e.user(t, "Nina Nosy")
	for _, slug := range []string{draft.Slug, archived.Slug, "does-not-exist"} {
		e.json(t, asker).PostJSON("/p/"+slug+"/questions", body).AssertNotFound()
	}

	// A suspended shop hides its products.
	e.exec(t, `UPDATE shops SET status = ? WHERE id = ?`, catalog.ShopSuspended, e.shop.ID)
	e.json(t, asker).PostJSON(e.askPath(), body).AssertNotFound()

	vukemtest.AssertDatabaseCount(t, e.app, "product_questions", 0)

	// Guests must log in.
	e.app.Client(t).AcceptJSON().PostJSON(e.askPath(), body).AssertUnauthorized()
}

func TestAskNotifiesTheOwner(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	asker := e.user(t, "Bruno Comprador")

	e.json(t, asker).PostJSON(e.askPath(), map[string]any{"body": "Does it come with a 220V plug?"}).
		AssertRedirect("/p/" + e.product.Slug)

	if got := e.mailJobs(t, e.owner.Email); got != 1 {
		t.Fatalf("queued mails = %d, want 1 for the owner", got)
	}

	e.runQueued(t)

	sent := e.app.Mails().SentTo(e.owner.Email)
	if len(sent) != 1 {
		t.Fatalf("mails to the owner = %d, want 1", len(sent))
	}

	if !strings.Contains(sent[0].Subject, "New question about Liquidificador Turbo") {
		t.Fatalf("subject %q, want the product title", sent[0].Subject)
	}

	if len(e.app.Mails().SentTo(asker.Email)) != 0 {
		t.Fatal("the asker got the owner's email")
	}
}

func TestAskFlagOff(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	flagtest.Set(t, e.app, flags.ProductQuestions, false)

	e.json(t, e.user(t, "Late Buyer")).PostJSON(e.askPath(), map[string]any{"body": "Can I still ask something?"}).
		AssertNotFound()
	vukemtest.AssertDatabaseCount(t, e.app, "product_questions", 0)
	e.app.Client(t).Inertia("").Get("/p/"+e.product.Slug).AssertOK().AssertInertia("Catalog/Product").
		PropEquals("qna.enabled", false)
}

func TestProductPageQuestions(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	asker := e.user(t, "Maria Clara Souza")
	base := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

	for i := range 12 {
		e.question(t, asker, "Question number "+strings.Repeat("q", i+1), base.Add(time.Duration(i)*time.Hour))
	}

	newest := e.question(t, asker, "The newest question of all", base.Add(48*time.Hour))
	e.exec(t, `UPDATE product_questions SET answer = ?, answered_at = ? WHERE id = ?`, "Yes, it does.", base, newest.ID)

	page := e.app.Client(t).Inertia("").Get("/p/" + e.product.Slug).AssertOK().AssertInertia("Catalog/Product")
	page.PropEquals("questions.total", float64(13)).PropEquals("questions.last_page", float64(2)).
		HasProp("questions.data", 10).
		PropEquals("questions.data.0.body", "The newest question of all").
		PropEquals("questions.data.0.answer", "Yes, it does.").
		PropEquals("questions.data.0.asker_name", "Maria").
		PropEquals("questions.data.1.answer", "").
		PropEquals("questions.data.1.answered_at", nil)

	// Privacy: only the first name reaches the page.
	body := e.app.Client(t).Inertia("").Get("/p/" + e.product.Slug).Body()
	if strings.Contains(body, "Souza") || strings.Contains(body, asker.Email) {
		t.Fatal("the product page shows the asker's surname or email")
	}

	e.app.Client(t).Inertia("").Get("/p/"+e.product.Slug+"?q_page=2").AssertOK().AssertInertia("Catalog/Product").
		PropEquals("questions.page", float64(2)).HasProp("questions.data", 3).
		PropEquals("questions.data.2.body", "Question number q")
}
