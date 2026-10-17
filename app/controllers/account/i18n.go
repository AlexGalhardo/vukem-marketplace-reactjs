package account

import (
	"context"
	"errors"
	"strings"

	"github.com/AlexGalhardo/vukem/errs"
	"github.com/AlexGalhardo/vukem/i18n"
	"github.com/AlexGalhardo/vukem/inertia"
	"github.com/AlexGalhardo/vukem/web"
)

// vukemKey maps an English message produced inside vukem (guards, ORM
// rules, social sign-in) to its catalog key.
//
// workaround(vukem): auth, orm.Unique and social errors are fixed English
// strings, not i18n keys, so they are matched here by text.
func vukemKey(msg string) (string, bool) {
	switch msg {
	case "These credentials do not match our records.":
		return "account.errors.credentials", true
	case "The email has already been taken.":
		return "account.errors.email_taken", true
	case "The sign-in attempt expired or is invalid. Please try again.":
		return "account.errors.social_state", true
	case "Sign-in was cancelled.":
		return "account.errors.social_denied", true
	case "Your email address is not verified.":
		return "account.errors.unverified", true
	}

	return "", false
}

// Localized translates the errors fn returns: messages that are catalog
// keys ("account.*") and the known vukem messages become text in the
// request locale. Handlers and the accounts package use keys only.
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

	if he, ok := errors.AsType[*errs.HTTPError](err); ok && he.Message != "" {
		if msg := translate(ctx, he.Message); msg != he.Message {
			// A copy: some errors are shared package variables.
			return &errs.HTTPError{Status: he.Status, Message: msg, Err: he.Err, Headers: he.Headers}
		}
	}

	return err
}

func translate(ctx context.Context, msg string) string {
	if key, ok := vukemKey(msg); ok {
		msg = key
	}

	if strings.HasPrefix(msg, "account.") {
		return i18n.T(ctx, msg)
	}

	return msg
}

// flash shows the translation of key on the next page.
func flash(c *web.Ctx, key string) {
	inertia.Flash(c, "status", i18n.T(c.Context(), key))
}
