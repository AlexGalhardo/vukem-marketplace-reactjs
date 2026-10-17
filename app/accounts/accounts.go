// Package accounts wires authentication: guards, tokens, password resets
// and email verification. It is your code (vukem install:auth): change the
// rules freely.
package accounts

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/auth"
	"github.com/AlexGalhardo/vukem/auth/hash"
	"github.com/AlexGalhardo/vukem/cache"
	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/mail"
	"github.com/AlexGalhardo/vukem/queue"
	"github.com/AlexGalhardo/vukem/signed"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/flags"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/models"
)

// Accounts holds the authentication services.
type Accounts struct {
	DB     *database.DB
	Mailer *mail.Mailer
	Users  *auth.ORMProvider[models.User]
	Web    *auth.SessionGuard[models.User]
	API    *auth.TokenGuard[models.User]
	Tokens *auth.Tokens
	Hasher hash.Hasher
	Signer *signed.Signer
	// Flags are the admin-managed feature flags (email verification, 2FA
	// enrolment, Google sign-in).
	Flags *flags.Flags
	// URL is the public base URL used in emails (APP_URL).
	URL string
	// ResetTTL is how long password reset links work.
	ResetTTL time.Duration
	// VerifyTTL is how long email verification links work.
	VerifyTTL time.Duration

	// mailLater renders a mail now and sends it from a queue job, so SMTP
	// latency and failures stay off the request path.
	mailLater func(ctx context.Context, m mail.Mailable, opts ...queue.Option) error
	queue     *queue.Queue
	cache     cache.Store
	now       func() time.Time
}

// New builds Accounts from the app.
func New(app *vukem.App) (*Accounts, error) {
	db, err := app.DB()
	if err != nil {
		return nil, err
	}

	mailer, err := app.Mailer()
	if err != nil {
		return nil, err
	}

	q, err := app.Queue()
	if err != nil {
		return nil, err
	}

	c, err := app.Cache()
	if err != nil {
		return nil, err
	}

	ff, err := flags.New(app)
	if err != nil {
		return nil, err
	}

	hasher := hash.Default()
	if app.Kind == vukem.KindTest {
		hasher = hash.Fake() // fast hashing keeps tests quick
	}

	users := auth.NewORMProvider[models.User](db)
	tokens := auth.NewTokens(db)

	web := auth.NewSessionGuard("web", users, hasher, auth.NewRememberStore(db))
	web.InsecureCookies = !app.Config.IsProduction() && !strings.HasPrefix(app.Config.URL, "https://")

	return &Accounts{
		DB: db, Mailer: mailer, Users: users, Web: web, Tokens: tokens, Hasher: hasher,
		API:       auth.NewTokenGuard("api", users, tokens),
		Flags:     ff,
		Signer:    signed.New(app.Key.Derive("signed.urls", 32)),
		URL:       strings.TrimSuffix(app.Config.URL, "/"),
		ResetTTL:  time.Hour,
		VerifyTTL: time.Hour,
		mailLater: app.MailLater,
		queue:     q,
		cache:     c.Store(),
		now:       app.Now,
	}, nil
}

// RequireVerified lets signed-in users with a verified email through.
// Guests go to the login page; unverified browsers (Inertia visits
// included) are redirected to the verification notice, JSON clients get
// 403. Checkout, selling and the admin area use it. While the
// email_verification flag is off, any signed-in user passes.
func (a *Accounts) RequireVerified(next http.Handler) http.Handler {
	verified := auth.Required(a.Web)(auth.Verified(a.Web)(next))
	signedIn := auth.Required(a.Web)(next)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.VerificationRequired(r.Context()) {
			verified.ServeHTTP(w, r)
			return
		}

		signedIn.ServeHTTP(w, r)
	})
}

// VerificationRequired reports whether the email_verification flag is on.
func (a *Accounts) VerificationRequired(ctx context.Context) bool {
	return a.Flags.Enabled(ctx, flags.EmailVerification)
}
