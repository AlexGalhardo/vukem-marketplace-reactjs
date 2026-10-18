package routes

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/AlexGalhardo/vukem"
	vapi "github.com/AlexGalhardo/vukem/api"
	"github.com/AlexGalhardo/vukem/apidocs"
	"github.com/AlexGalhardo/vukem/auth"
	vgrpc "github.com/AlexGalhardo/vukem/contrib/grpc"
	"github.com/AlexGalhardo/vukem/middleware"
	"github.com/AlexGalhardo/vukem/openapi"
	"github.com/AlexGalhardo/vukem/ratelimit"
	"github.com/AlexGalhardo/vukem/router"
	"github.com/AlexGalhardo/vukem/web"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/accounts"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/api"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/api/sqlstore"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/controllers/apikeys"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/flags"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/models"
)

// Rate limits per minute; API_RATE_LIMIT tunes the per-caller one.
const (
	defaultAPIRateLimit = 120
	apiIPRateLimit      = 300
)

// API registers the public APIs (PLAN 3.8): REST /api/v1 with its OpenAPI
// document and Scalar reference at /docs/api, GraphQL /graphql, gRPC on
// GRPC_PORT, and the account page that manages API keys.
func API(r *router.Router, app *vukem.App) error {
	st, err := newAPIStack(app)
	if err != nil {
		return err
	}

	cache, err := app.Cache()
	if err != nil {
		return err
	}

	idempotency := vapi.Idempotency(cache.Store(), vapi.IdempotencyOptions{Scope: api.IdempotencyScope})
	rest := &api.REST{Svc: st.svc, Idempotency: idempotency}

	r.Group("/api/v1", func(g *router.Group) {
		g.Use(auth.ProblemJSON, middleware.JSONOnly(), st.ipLimit, st.authn.Middleware, st.callerLimit)
		rest.Register(g)
	}).Name("api.v1.")

	// The API is public, so its document and reference are served in
	// production too.
	vukem.ServeOpenAPIWith(app, r, "/api/v1/openapi.json",
		vukem.OpenAPIOptions{PathPrefix: "/api/v1", InProduction: true}).Name("api.v1.openapi")
	apidocs.Register(r, "/docs/api", apidocs.Config{
		SpecURL: "/api/v1/openapi.json", Title: app.Config.Name + " API",
		Authentication: map[string]any{"preferredSecurityScheme": "bearer"},
	})

	schema, err := api.NewGraphQL(st.svc, app.Log)
	if err != nil {
		return err
	}

	introspection := api.GraphQLIntrospection(
		app.Config.IsProduction(),
		app.Source.Get("GRAPHQL_INSECURE_INTROSPECTION"),
	)
	r.Handle("/graphql", api.GraphQLHandler(schema, introspection)).
		Use(auth.ProblemJSON, st.acc.Flags.Require(flags.GraphQLAPI), st.ipLimit, st.authn.Middleware, st.callerLimit).
		Name("graphql").Doc(openapi.Skip{})

	rpc, err := st.rpc().Handler(app)
	if err != nil {
		return err
	}

	if err := serveGRPC(app, rpc); err != nil {
		return err
	}

	pages := &apikeys.Handlers{Keys: st.keys, Now: app.Now}
	required := auth.Required(st.acc.Web)
	writes := ratelimit.Middleware(st.limiters("api-keys-page", 20, time.Minute), apiKeysPageKey)

	r.Get(apikeys.Path, pages.Show).Name("account.api-keys").Use(required)
	r.Post(apikeys.Path, pages.Store).Name("account.api-keys.store").Use(required, writes)
	r.Delete(apikeys.Path+"/{id}", pages.Destroy).Name("account.api-keys.destroy").Use(required, writes)

	return nil
}

// apiKeysPageKey buckets key creation and revocation per signed-in user.
func apiKeysPageKey(r *http.Request) string {
	u, err := auth.UserFrom[models.User](r.Context(), "web")
	if err != nil {
		return "ip:" + ratelimit.ByIP(r)
	}

	return "user:" + strconv.FormatInt(u.ID, 10)
}

// apiStack is what REST, GraphQL and gRPC share: services, API keys and
// the rate limits (one budget per caller across the three APIs).
type apiStack struct {
	acc         *accounts.Accounts
	limiters    ratelimit.Factory
	keys        *api.Keys
	svc         api.Services
	authn       *api.Authenticator
	keysOnly    *api.Authenticator
	ipLimit     router.Middleware
	callerLimit router.Middleware
}

func newAPIStack(app *vukem.App) (*apiStack, error) {
	acc, err := accounts.New(app)
	if err != nil {
		return nil, err
	}

	// RATE_LIMIT_STORE=cache shares the buckets across instances (Redis).
	limiters, err := app.RateLimiters()
	if err != nil {
		return nil, err
	}

	perCaller := defaultAPIRateLimit
	if v := app.Source.Get("API_RATE_LIMIT"); v != "" {
		if perCaller, err = strconv.Atoi(v); err != nil || perCaller < 1 {
			return nil, fmt.Errorf("API_RATE_LIMIT must be a positive integer, got %q", v)
		}
	}

	keys := api.NewKeys(acc.DB, app.Now)

	return &apiStack{
		acc:      acc,
		limiters: limiters,
		keys:     keys,
		svc:      sqlstore.New(acc.DB, app.Now).Services(),
		authn: &api.Authenticator{Keys: keys, Log: app.Log, Session: func(c *web.Ctx) (int64, bool) {
			u, err := acc.Web.User(c)
			return u.ID, err == nil
		}},
		keysOnly: &api.Authenticator{Keys: keys, Log: app.Log},
		// The IP limit runs before authentication so key guessing is
		// throttled too; the per-caller limit runs after it (per key, user
		// or IP).
		ipLimit:     ratelimit.Middleware(limiters("api-ip", apiIPRateLimit, time.Minute), ratelimit.ByIP),
		callerLimit: ratelimit.Middleware(limiters("api", perCaller, time.Minute), api.RateKey),
	}, nil
}

func (st *apiStack) rpc() *api.RPC {
	return &api.RPC{Svc: st.svc, Middleware: []router.Middleware{st.ipLimit, st.keysOnly.Middleware, st.callerLimit}}
}

// RPCHandler builds the handler the gRPC port serves, with a stack of its
// own (tests serve it in process; API wires the one it serves).
func RPCHandler(app *vukem.App) (http.Handler, error) {
	st, err := newAPIStack(app)
	if err != nil {
		return nil, err
	}

	return st.rpc().Handler(app)
}

// serveGRPC runs the RPC router on GRPC_PORT next to the HTTP server,
// drained with the app.
func serveGRPC(app *vukem.App, h http.Handler) error {
	if app.Kind != vukem.KindWeb {
		return nil
	}

	run, addr, err := api.GRPCPortFrom(app.Source).Runner(&app.Config, h)

	switch {
	// Fail closed by not serving gRPC, rather than by refusing to boot: the
	// web app must stay deployable while the gRPC certificate is pending.
	case errors.Is(err, vgrpc.ErrCleartextInProduction):
		app.Log.Error("gRPC disabled: production needs GRPC_TLS_CERT_FILE and GRPC_TLS_KEY_FILE, " +
			"or GRPC_INSECURE_CLEARTEXT=true when a proxy terminates TLS")

		return nil
	case err != nil:
		return err
	case run == nil:
		return nil
	}

	tls := app.Source.Get("GRPC_TLS_CERT_FILE") != "" && app.Source.Get("GRPC_TLS_KEY_FILE") != ""

	app.AddRunner("grpc", vukem.RunnerFunc(func(ctx context.Context) error {
		app.Log.InfoContext(ctx, "grpc server starting", "addr", addr, "tls", tls)
		return run.Run(ctx)
	}))

	return nil
}
