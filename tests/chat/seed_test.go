package chat_test

import (
	"testing"

	"github.com/AlexGalhardo/vukem/orm"
	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/models"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/database/seeders"
)

func (e *env) demoUser(t *testing.T, email string) models.User {
	t.Helper()

	u := models.User{Name: "Demo " + email, Email: email, Password: "x", Role: models.RoleUser}
	if err := orm.Create(t.Context(), db(t, e.app), &u); err != nil {
		t.Fatal(err)
	}

	return u
}

func TestDemoChatSkipsWithoutDemoAccounts(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.user(t, "Real Buyer")

	n, err := seeders.DemoChat{}.Seed(t.Context(), db(t, e.app))
	if err != nil || n != 0 {
		t.Fatalf("seed = %d, %v; want 0 and no error", n, err)
	}

	vukemtest.AssertDatabaseCount(t, e.app, "conversations", 0)
}

func TestDemoChatSeedsOnlyDemoAccountsOnce(t *testing.T) {
	t.Parallel()

	e := newEnv(t)

	for _, email := range []string{"ana@compradores.example.com", "bruno@compradores.example.com"} {
		e.demoUser(t, email)
	}

	for i, email := range []string{"loja1@vendedores.example.com", "loja2@vendedores.example.com", "loja3@vendedores.example.com"} {
		s := e.demoUser(t, email)
		e.sellerWithProduct(t, s, "Loja Demo "+itoa(int64(i)), "Produto Demo "+itoa(int64(i)))
	}

	n, err := seeders.DemoChat{}.Seed(t.Context(), db(t, e.app))
	if err != nil || n != 2 {
		t.Fatalf("seed = %d, %v; want 2 (one per demo buyer)", n, err)
	}

	again, err := seeders.DemoChat{}.Seed(t.Context(), db(t, e.app))
	if err != nil || again != 0 {
		t.Fatalf("second seed = %d, %v; want 0", again, err)
	}

	vukemtest.AssertDatabaseCount(t, e.app, "conversations", 2)
	vukemtest.AssertDatabaseCount(t, e.app, "chat_messages", 3+4)

	// The env's real seller (Olga) got nothing.
	e.inertia(t, e.owner).Get("/seller/messages").AssertOK().AssertInertia("Messages/Index").
		HasProp("conversations.data", 0)
}
