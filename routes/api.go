package routes

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/AlexGalhardo/vukem"
	vapi "github.com/AlexGalhardo/vukem/api"
	"github.com/AlexGalhardo/vukem/apidocs"
	"github.com/AlexGalhardo/vukem/auth"
	"github.com/AlexGalhardo/vukem/middleware"
	"github.com/AlexGalhardo/vukem/openapi"
	"github.com/AlexGalhardo/vukem/ratelimit"
	"github.com/AlexGalhardo/vukem/router"
	"github.com/AlexGalhardo/vukem/web"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/accounts"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/api"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/api/sqlstore"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/controllers/apikeys"
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
	acc, err := accounts.New(app)
	if err != nil {
		return err
	}

	cache, err := app.Cache()
	if err != nil {
		return err
	}

	// RATE_LIMIT_STORE=cache shares the buckets across instances (Redis).
	limiters, err := app.RateLimiters()
	if err != nil {
		return err
	}

	perCaller := defaultAPIRateLimit
	if v := app.Source.Get("API_RATE_LIMIT"); v != "" {
		if perCaller, err = strconv.Atoi(v); err != nil || perCaller < 1 {
			return fmt.Errorf("API_RATE_LIMIT must be a positive integer, got %q", v)
		}
	}

	keys := api.NewKeys(acc.DB, app.Now)
	svc := sqlstore.New(acc.DB, app.Now).Services()

	authn := &api.Authenticator{Keys: keys, Log: app.Log, Session: func(c *web.Ctx) (int64, bool) {
		u, err := acc.Web.User(c)
		return u.ID, err == nil
	}}
	keysOnly := &api.Authenticator{Keys: keys, Log: app.Log}

	// The IP limit runs before authentication so key guessing is throttled
	// too; the per-caller limit runs after it (per key, user or IP).
	ipLimit := ratelimit.Middleware(limiters("api-ip", apiIPRateLimit, time.Minute), ratelimit.ByIP)
	callerLimit := ratelimit.Middleware(limiters("api", perCaller, time.Minute), api.RateKey)
	idempotency := vapi.Idempotency(cache.Store(), vapi.IdempotencyOptions{Scope: api.IdempotencyScope})

	rest := &api.REST{Svc: svc, Idempotency: idempotency}

	r.Group("/api/v1", func(g *router.Group) {
		g.Use(middleware.JSONOnly(), ipLimit, authn.Middleware, callerLimit)
		rest.Register(g)
	}).Name("api.v1.")

	// The API is public, so its document and reference are served in
	// production too (vukem.ServeOpenAPI would need OPENAPI_IN_PRODUCTION).
	r.Get("/api/v1/openapi.json", func(c *web.Ctx) error {
		return c.JSON(http.StatusOK, openapi.Generate(app.Router.Routes(), openapi.Info{
			Title: app.Config.Name + " API", Version: "v1", Servers: []string{app.Config.URL}, PathPrefix: "/api/v1",
		}))
	}).Name("api.v1.openapi").Doc(openapi.Skip{})
	apidocs.Register(r, "/docs/api", apidocs.Config{
		SpecURL: "/api/v1/openapi.json", Title: app.Config.Name + " API",
		Authentication: map[string]any{"preferredSecurityScheme": "bearer"},
	})

	schema, err := api.NewGraphQL(svc, app.Log)
	if err != nil {
		return err
	}

	introspection := api.GraphQLIntrospection(
		app.Config.IsProduction(),
		app.Source.Get("GRAPHQL_INSECURE_INTROSPECTION"),
	)
	r.Handle("/graphql", api.GraphQLHandler(schema, introspection)).
		Use(ipLimit, authn.Middleware, callerLimit).Name("graphql").Doc(openapi.Skip{})

	rpc := &api.RPC{Svc: svc, Middleware: []router.Middleware{ipLimit, keysOnly.Middleware, callerLimit}}
	rpc.Register(app, r)

	if err := grpcRunner(app); err != nil {
		return err
	}

	pages := &apikeys.Handlers{Keys: keys, Now: app.Now}
	required := auth.Required(acc.Web)
	writes := ratelimit.Middleware(limiters("api-keys-page", 20, time.Minute), apiKeysPageKey)

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

// grpcRunner serves the RPC routes on GRPC_PORT (h2c, or TLS with
// GRPC_TLS_CERT_FILE/GRPC_TLS_KEY_FILE), started and drained with the app.
// API keys travel in metadata, so production refuses plaintext unless
// GRPC_INSECURE_PLAINTEXT=true (TLS terminated in front of the app).
func grpcRunner(app *vukem.App) error {
	port := app.Source.Get("GRPC_PORT")
	if app.Kind != vukem.KindWeb || port == "off" {
		return nil
	}

	if port == "" {
		port = "50051"
	}

	certFile, keyFile := app.Source.Get("GRPC_TLS_CERT_FILE"), app.Source.Get("GRPC_TLS_KEY_FILE")
	useTLS := certFile != "" && keyFile != ""

	if app.Config.IsProduction() && !useTLS && app.Source.Get("GRPC_INSECURE_PLAINTEXT") != "true" {
		return errors.New("gRPC needs TLS in production: set GRPC_TLS_CERT_FILE and GRPC_TLS_KEY_FILE, " +
			"GRPC_INSECURE_PLAINTEXT=true when a proxy terminates TLS, or GRPC_PORT=off")
	}

	host := app.Source.Get("HOST")
	addr := net.JoinHostPort(host, port)

	app.AddRunner("grpc", vukem.RunnerFunc(func(ctx context.Context) error {
		h, err := app.Handler()
		if err != nil {
			return err
		}

		srv := vukem.NewHTTPServer(addr, api.GRPCListener(h))
		srv.ShutdownTimeout = app.Config.ShutdownTimeout

		var protocols http.Protocols
		protocols.SetHTTP1(true)
		protocols.SetUnencryptedHTTP2(!useTLS)
		protocols.SetHTTP2(useTLS)
		srv.Server.Protocols = &protocols

		if useTLS {
			cert, err := tls.LoadX509KeyPair(certFile, keyFile)
			if err != nil {
				return fmt.Errorf("grpc tls: %w", err)
			}

			ln, err := tls.Listen("tcp", addr, &tls.Config{
				Certificates: []tls.Certificate{
					cert,
				},
				MinVersion: tls.VersionTLS12,
				NextProtos: []string{"h2", "http/1.1"},
			})
			if err != nil {
				return fmt.Errorf("grpc listen: %w", err)
			}

			srv.Listener = ln
		}

		app.Log.InfoContext(ctx, "grpc server starting", "addr", addr, "tls", useTLS)

		return srv.Run(ctx)
	}))

	return nil
}
