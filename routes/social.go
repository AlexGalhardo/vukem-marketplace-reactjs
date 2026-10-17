package routes

import (
	"github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/auth"
	"github.com/AlexGalhardo/vukem/router"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/accounts"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/controllers/account"
)

// Social registers "sign in with" routes (vukem install:auth --social).
func Social(r *router.Router, app *vukem.App) error {
	acc, err := accounts.New(app)
	if err != nil {
		return err
	}

	L := account.Localized
	s, err := accounts.NewSocial(app, "google")
	if err != nil {
		return err
	}

	h := account.NewSocial(acc, s)

	r.Get("/auth/{provider}/redirect", L(h.Redirect)).Name("social.redirect").Use(auth.Guest(acc.Web))
	r.Get("/auth/{provider}/callback", L(h.Callback)).Name("social.callback")
	// Apple answers with response_mode=form_post.
	r.Post("/auth/{provider}/callback", L(h.Callback))

	return nil
}
