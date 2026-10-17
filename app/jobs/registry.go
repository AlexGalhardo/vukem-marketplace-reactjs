// Package jobs holds the queued jobs.
package jobs

import (
	"context"
	"sync"
	"time"

	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/i18n"
	"github.com/AlexGalhardo/vukem/queue"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/accounts"
)

// Binding hands the app to the job handlers. Registrations are passed to
// vukem.New, before the app exists, so bootstrap calls Bind right after;
// handlers only run later (worker or sync dispatch).
type Binding struct {
	app *vukem.App

	once sync.Once
	acc  *accounts.Accounts
	err  error
}

// Bind sets the app the handlers use.
func (b *Binding) Bind(app *vukem.App) { b.app = app }

func (b *Binding) accounts() (*accounts.Accounts, error) {
	b.once.Do(func() { b.acc, b.err = accounts.New(b.app) })
	return b.acc, b.err
}

// localized returns ctx with the app's translations in locale, so mails
// rendered by a worker speak the language of the request that queued them.
func (b *Binding) localized(ctx context.Context, locale string) context.Context {
	if b.app.I18n == nil {
		return ctx
	}

	if l, ok := b.app.I18n.Match(locale); ok {
		locale = l
	} else {
		locale = b.app.I18n.Default()
	}

	return i18n.WithLocale(i18n.WithBundle(ctx, b.app.I18n), locale)
}

// All lists the job handlers; vukem make:job appends here.
func All(b *Binding) []queue.Registration {
	return []queue.Registration{
		queue.Handle(func(ctx context.Context, j accounts.PasswordResetJob) error {
			acc, err := b.accounts()
			if err != nil {
				return err
			}

			return acc.SendResetLink(b.localized(ctx, j.Locale), j.Email)
		}, queue.Retries(3), queue.Backoff(queue.Exponential(10*time.Second))),
	}
}
