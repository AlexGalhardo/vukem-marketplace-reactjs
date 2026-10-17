package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/AlexGalhardo/vukem/errs"
	"github.com/AlexGalhardo/vukem/ratelimit"
	"github.com/AlexGalhardo/vukem/web"
)

// Principal is who calls the API: a user through one of their API keys, or
// a user signed in with the session cookie (all scopes).
type Principal struct {
	UserID int64
	// KeyID is zero for session callers.
	KeyID  int64
	Scopes []string
}

// Can reports whether p holds scope.
func (p Principal) Can(scope string) bool { return slices.Contains(p.Scopes, scope) }

// IsKey reports whether p authenticated with an API key.
func (p Principal) IsKey() bool { return p.KeyID != 0 }

// ID identifies p for rate limits and idempotency keys: "key:<id>" or
// "user:<id>", never empty.
func (p Principal) ID() string {
	if p.IsKey() {
		return "key:" + strconv.FormatInt(p.KeyID, 10)
	}

	return "user:" + strconv.FormatInt(p.UserID, 10)
}

type principalKey struct{}

// WithPrincipal returns ctx carrying p.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFrom returns the caller authenticated for this request.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

// Client-facing auth errors; one message for every key failure.
var (
	errUnauthenticated = errs.New(http.StatusUnauthorized, "A valid API key or session is required.").
				WithHeader("WWW-Authenticate", `Bearer realm="api"`)
	errInvalidKey = errs.New(http.StatusUnauthorized, "Invalid API key.").
			WithHeader("WWW-Authenticate", `Bearer realm="api", error="invalid_token"`)
)

func errMissingScope(scope string) error {
	return errs.New(http.StatusForbidden, "This API key lacks the "+scope+" scope.")
}

// SessionUser returns the user signed in with the session cookie.
type SessionUser func(c *web.Ctx) (int64, bool)

// Authenticator resolves the Principal of API requests.
type Authenticator struct {
	Keys *Keys
	// Session authenticates cookie callers; nil accepts API keys only.
	Session SessionUser
	Log     *slog.Logger
}

// Middleware authenticates the request when it carries credentials and
// never blocks anonymous ones (Require does). A bad Authorization header
// is always a 401, even with a valid session cookie next to it.
func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := web.NewCtx(w, r, web.ServicesFrom(r.Context()))

		if header := r.Header.Get("Authorization"); header != "" {
			p, err := a.fromKey(r.Context(), header)
			if err != nil {
				if !errors.Is(err, ErrInvalidKey) {
					fail(w, r, err)
					return
				}

				web.SecurityEvent(r, "api.key_rejected")
				fail(w, r, errInvalidKey)

				return
			}

			next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), p)))

			return
		}

		if a.Session != nil {
			if id, ok := a.Session(c); ok {
				p := Principal{UserID: id, Scopes: AllScopes()}
				next.ServeHTTP(w, c.Request().WithContext(WithPrincipal(c.Request().Context(), p)))

				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func (a *Authenticator) fromKey(ctx context.Context, header string) (Principal, error) {
	scheme, token, _ := strings.Cut(header, " ")
	if !strings.EqualFold(scheme, "Bearer") {
		return Principal{}, ErrInvalidKey
	}

	key, err := a.Keys.Authenticate(ctx, strings.TrimSpace(token))
	if err != nil {
		return Principal{}, err
	}

	// A failed last_used_at write must not fail the request.
	if err := a.Keys.Touch(context.WithoutCancel(ctx), key); err != nil && a.Log != nil {
		a.Log.WarnContext(ctx, "api key touch failed", "key_id", key.ID, "err", err)
	}

	return Principal{UserID: key.UserID, KeyID: key.ID, Scopes: key.Scopes}, nil
}

// Require answers 401 without a Principal and 403 when it lacks scope.
func Require(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := PrincipalFrom(r.Context())

			switch {
			case !ok:
				fail(w, r, errUnauthenticated)
			case !p.Can(scope):
				fail(w, r, errMissingScope(scope))
			default:
				next.ServeHTTP(w, r)
			}
		})
	}
}

// Allow lets anonymous callers through (public endpoints) but refuses an
// API key that lacks scope.
func Allow(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if p, ok := PrincipalFrom(r.Context()); ok && !p.Can(scope) {
				fail(w, r, errMissingScope(scope))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// fail renders an auth error as problem+json whatever the Accept header
// says, like vukem's APIGuard failures: API routes never answer HTML.
// Only the error path sees the forced Accept.
func fail(w http.ResponseWriter, r *http.Request, err error) {
	// workaround(vukem): auth cannot force problem+json for custom API authenticators
	r = r.Clone(r.Context())
	r.Header.Set("Accept", "application/json")
	web.NewCtx(w, r, web.ServicesFrom(r.Context())).HandleError(err)
}

// RateKey buckets requests per API key, per session user, or per IP for
// anonymous callers.
func RateKey(r *http.Request) string {
	if p, ok := PrincipalFrom(r.Context()); ok {
		return p.ID()
	}

	return "ip:" + ratelimit.ByIP(r)
}

// IdempotencyScope scopes Idempotency-Key replays to the caller; anonymous
// requests get "" and are refused when they send a key.
func IdempotencyScope(r *http.Request) string {
	if p, ok := PrincipalFrom(r.Context()); ok {
		return p.ID()
	}

	return ""
}
