package controllers

import (
	"net/http"
	"time"

	"github.com/AlexGalhardo/vukem/i18n"
	"github.com/AlexGalhardo/vukem/web"
	"github.com/AlexGalhardo/vukem/z"
)

type localeInput struct {
	Locale string `json:"locale"`
}

var localeSchema = z.Struct[localeInput](z.Shape{
	"locale": z.String().Trim().Min(2).Max(10),
})

// SetLocale stores the chosen locale in the "lang" cookie read by the
// i18n middleware, then redirects back. Unknown locales are rejected.
// secure is false only in local development over plain HTTP.
func SetLocale(bundle *i18n.Bundle, secure bool) web.HandlerFunc {
	return func(c *web.Ctx) error {
		in, err := web.Parse(c, localeSchema)
		if err != nil {
			return err
		}

		locale, ok := bundle.Match(in.Locale)
		if !ok {
			return c.Problem(http.StatusUnprocessableEntity, "Unsupported locale", "")
		}

		ck := &http.Cookie{
			Name:     "lang",
			Value:    locale,
			Path:     "/",
			MaxAge:   int((365 * 24 * time.Hour).Seconds()),
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		}
		// Browsers drop Secure cookies on http://localhost in development.
		ck.Secure = secure
		c.SetCookie(ck)

		return c.Back("/")
	}
}
