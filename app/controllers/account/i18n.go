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

// Localized translates the errors fn returns whose messages are our
// catalog keys ("account.*") into the request locale. vukem's own auth,
// social and validation errors are keyed and translated by vukem.
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
	if strings.HasPrefix(msg, "account.") {
		return i18n.T(ctx, msg)
	}

	return msg
}

// flash shows the translation of key on the next page.
func flash(c *web.Ctx, key string) {
	inertia.Flash(c, "status", i18n.T(c.Context(), key))
}
