package chat_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/catalog"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/flags"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/internal/testing/flagtest"
)

func TestStartIsOnePerBuyerShopAndProduct(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	buyer := e.user(t, "Bia Buyer")

	first := e.start(t, buyer)
	if again := e.start(t, buyer); again != first {
		t.Fatalf("second start opened %d, want the same conversation %d", again, first)
	}

	// The same shop without a product (from an order) is another thread.
	res := e.json(t, buyer).PostJSON("/messages", map[string]any{"shop": e.shop.Slug})
	if loc := res.Header("Location"); loc == "" || loc == path(first) {
		t.Fatalf("shop conversation location %q, want a new one", loc)
	}

	vukemtest.AssertDatabaseCount(t, e.app, "conversations", 2)
}

func TestStartRefusals(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	buyer := e.user(t, "Bia Buyer")

	// The shop owner cannot chat with their own shop.
	own := e.json(t, e.owner).PostJSON("/messages", map[string]any{"product": e.product.Slug})
	own.AssertForbidden()

	if !strings.Contains(own.Body(), "your own shop") {
		t.Fatalf("own shop body %s, want the friendly message", own.Body())
	}

	draft := e.catalog(t)
	p, err := draft.CreateProduct(t.Context(), e.owner.ID, catalog.ProductInput{
		Title: "Rascunho", Description: "x", PriceCents: 100, Stock: 1, WeightGrams: 10,
	})
	if err != nil {
		t.Fatal(err)
	}

	for name, body := range map[string]map[string]any{
		"unknown product": {"product": "nope"},
		"draft product":   {"product": p.Slug},
		"unknown shop":    {"shop": "nope"},
		"both":            {"product": e.product.Slug, "shop": e.shop.Slug},
		"neither":         {},
	} {
		if res := e.json(t, buyer).PostJSON("/messages", body); res.Status() != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", name, res.Status())
		}
	}

	vukemtest.AssertDatabaseCount(t, e.app, "conversations", 0)

	// Guests are sent to log in.
	e.app.Client(t).AcceptJSON().PostJSON("/messages", map[string]any{"product": e.product.Slug}).
		AssertUnauthorized()
}

func TestSendValidation(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	buyer := e.user(t, "Bia Buyer")
	id := e.start(t, buyer)

	cases := []struct {
		name string
		body string
		ok   bool
	}{
		{"empty", "", false},
		{"spaces only", "   \n ", false},
		{"one char", "a", true},
		{"2000 chars", strings.Repeat("d", 2000), true},
		{"2000 runes", strings.Repeat("é", 2000), true},
		{"2001 chars", strings.Repeat("e", 2001), false},
	}
	for _, tc := range cases {
		res := e.json(t, buyer).PostJSON(path(id), map[string]any{"body": tc.body})
		if tc.ok {
			res.AssertCreated()
			continue
		}

		res.AssertUnprocessable().AssertJSONValidationErrors("body")
	}

	vukemtest.AssertDatabaseCount(t, e.app, "chat_messages", 3)
}

func TestSendRejectsContactData(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	buyer := e.user(t, "Bia Buyer")
	id := e.start(t, buyer)

	for _, body := range []string{
		"Me chama no zap (11) 98765-4321",
		"meu email é ana.souza@example.com",
		"compra direto em www.minhaloja.com",
		"me segue @loja_da_ana",
	} {
		res := e.json(t, buyer).PostJSON(path(id), map[string]any{"body": body})
		res.AssertUnprocessable().AssertJSONValidationErrors("body")

		if !strings.Contains(res.Body(), "keep the conversation on the platform") {
			t.Fatalf("%q: body %s, want the friendly no-contact message", body, res.Body())
		}
	}

	pt := e.json(t, buyer).WithCookie("lang", "pt-BR").PostJSON(path(id), map[string]any{"body": "liga 11 99999-8888"})
	pt.AssertUnprocessable()

	if !strings.Contains(pt.Body(), "mantenha a conversa na plataforma") {
		t.Fatalf("pt-BR body %s, want the Portuguese message", pt.Body())
	}

	vukemtest.AssertDatabaseCount(t, e.app, "chat_messages", 0)
}

func TestMessagesArePlainText(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	buyer := e.user(t, "Bia Buyer")
	id := e.start(t, buyer)
	body := `<img src=x onerror=alert(1)> & <b>bold</b>`

	e.send(t, buyer, id, body)

	// Stored as typed; the page receives it as a JSON string (React renders
	// text, never HTML).
	vukemtest.AssertDatabaseHas(t, e.app, "chat_messages", map[string]any{"body": body})
	e.inertia(t, e.owner).Get(path(id)).AssertOK().AssertInertia("Messages/Show").
		PropEquals("messages.0.body", body)
}

func TestSendRateLimit(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	buyer := e.user(t, "Rita Rapid")
	id := e.start(t, buyer)
	c := e.json(t, buyer)

	for i := range 30 {
		c.PostJSON(path(id), map[string]any{"body": "message " + strings.Repeat("x", i+1)}).AssertCreated()
	}

	c.PostJSON(path(id), map[string]any{"body": "one too many"}).AssertStatus(http.StatusTooManyRequests)

	// The budget is per user: the seller still answers.
	e.json(t, e.owner).PostJSON(path(id), map[string]any{"body": "Olá! Como posso ajudar?"}).AssertCreated()
}

// TestOutsidersCannotReachAConversation is the IDOR matrix: a signed-in
// user who is neither the buyer nor the shop owner gets 404 everywhere, as
// if the conversation did not exist.
func TestOutsidersCannotReachAConversation(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	buyer := e.user(t, "Bia Buyer")
	mallory := e.user(t, "Mallory")
	id := e.start(t, buyer)
	msg := e.send(t, buyer, id, "Tem garantia?")

	c := e.json(t, mallory)
	c.Get(path(id)).AssertNotFound()
	c.PostJSON(path(id), map[string]any{"body": "intruding"}).AssertNotFound()
	c.Get(path(id, "/since?after=0")).AssertNotFound()
	c.PostJSON(path(id, "/read"), map[string]any{"last_id": msg}).AssertNotFound()
	c.PostJSON(path(id, "/typing"), nil).AssertNotFound()
	c.Get(path(id + 1000)).AssertNotFound()
	c.Get("/messages/abc").AssertNotFound()

	// Mallory's own seller inbox (she has a shop) does not list it.
	e.sellerWithProduct(t, mallory, "Loja da Mallory", "Ventilador")
	e.inertia(t, mallory).Get("/seller/messages").AssertOK().AssertInertia("Messages/Index").
		HasProp("conversations.data", 0)

	vukemtest.AssertDatabaseCount(t, e.app, "chat_messages", 1)
	vukemtest.AssertDatabaseHas(t, e.app, "conversations", map[string]any{"id": id, "seller_last_read_id": 0})
}

func TestConversationFlowAndReadReceipts(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	buyer := e.user(t, "Bia Buyer")
	id := e.start(t, buyer)

	m1 := e.send(t, buyer, id, "Oi, tem na cor azul?")
	m2 := e.send(t, buyer, id, "E entrega em Recife?")

	// The seller sees two unread messages in the header badge and the inbox.
	page := e.inertia(t, e.owner).Get("/seller/messages").AssertOK().AssertInertia("Messages/Index")
	page.PropEquals("chat.unread", 2).PropEquals("conversations.data.0.unread", 2).
		PropEquals("conversations.data.0.with", "Bia").PropEquals("conversations.data.0.product.slug", e.product.Slug)

	// Opening the conversation marks it read.
	e.inertia(t, e.owner).Get(path(id)).AssertOK().AssertInertia("Messages/Show").
		PropEquals("conversation.role", "seller").PropEquals("conversation.with", "Bia").
		HasProp("messages", 2).PropEquals("conversation.my_last_read_id", m2)
	e.inertia(t, e.owner).Get("/seller").AssertOK().AssertInertia("Seller/Dashboard").PropEquals("chat.unread", 0)

	m3 := e.send(t, e.owner, id, "Tem sim! Entregamos em todo o Brasil.")

	// The buyer catches up after m2; sending marked the seller's receipt.
	since := e.json(t, buyer).Get(path(id, "/since?after=", itoa(m2))).AssertOK()
	since.AssertJSONCount("messages", 1).AssertJSONPath("messages.0.id", m3).
		AssertJSONPath("their_last_read_id", m3)

	// Receipts never move back and never pass the newest message.
	c := e.json(t, buyer)
	c.PostJSON(path(id, "/read"), map[string]any{"last_id": m3 + 50}).AssertOK().
		AssertJSONPath("last_read_id", m3).AssertJSONPath("unread", 0)
	c.PostJSON(path(id, "/read"), map[string]any{"last_id": m1}).AssertOK().AssertJSONPath("last_read_id", m3)
	c.PostJSON(path(id, "/read"), map[string]any{"last_id": 0}).AssertUnprocessable()

	// The buyer inbox lists it with the shop's name.
	e.inertia(t, buyer).Get("/messages").AssertOK().AssertInertia("Messages/Index").
		PropEquals("role", "buyer").PropEquals("conversations.data.0.with", "Loja da Olga").
		PropEquals("conversations.data.0.last_from_me", false)
}

func TestSellerInboxNeedsAShop(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	buyer := e.user(t, "Bia Buyer")

	e.inertia(t, buyer).Get("/seller/messages").AssertRedirect("/sell")
}

func TestChatFlagOffHidesEverything(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	buyer := e.user(t, "Bia Buyer")
	id := e.start(t, buyer)

	flagtest.Set(t, e.app, flags.Chat, false)

	c := e.json(t, buyer)
	c.Get("/messages").AssertNotFound()
	c.Get(path(id)).AssertNotFound()
	c.PostJSON(path(id), map[string]any{"body": "hello"}).AssertNotFound()
	c.PostJSON("/messages", map[string]any{"product": e.product.Slug}).AssertNotFound()
	c.Get("/broadcast/ws").AssertNotFound()
	c.Get("/broadcast/sse?channels=" + "private-chat." + itoa(id)).AssertNotFound()

	e.inertia(t, buyer).Get("/p/"+e.product.Slug).AssertOK().AssertInertia("Catalog/Product").
		PropEquals("chat.enabled", false)
}

func TestAdminReadsConversationsReadOnly(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	buyer := e.user(t, "Bia Buyer")
	id := e.start(t, buyer)
	e.send(t, buyer, id, "Pergunta para moderação")

	admin := e.admin(t)
	e.inertia(t, admin).Get("/admin/messages").AssertOK().AssertInertia("Admin/Messages").
		HasProp("conversations.data", 1)
	e.inertia(t, admin).Get("/admin/messages/"+itoa(id)).AssertOK().AssertInertia("Admin/Conversation").
		HasProp("messages", 1)

	// The admin is not a participant: no sending, no page as a participant.
	e.json(t, admin).PostJSON(path(id), map[string]any{"body": "admin here"}).AssertNotFound()

	// Non-admins do not see the admin area.
	e.json(t, buyer).Get("/admin/messages").AssertNotFound()
	e.json(t, buyer).Get("/admin/messages/" + itoa(id)).AssertNotFound()
}
