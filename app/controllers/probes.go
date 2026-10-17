package controllers

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// Probes answers the platform probes: /health (liveness: the process
// serves requests) and /ready (readiness: Postgres, Redis and MongoDB
// answer). Railway's healthcheck points at /ready.
type Probes struct {
	// Ready is the readiness handler (app.ReadyHandler: every registered
	// health check, details only in the log).
	Ready   http.Handler
	Log     *slog.Logger
	Version string
}

// Middleware serves the probes at the server level, before routing, so
// probes skip sessions, CSRF, i18n and Inertia entirely: cheap and
// stateless even under aggressive health checking.
func (p Probes) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}

		switch r.URL.Path {
		case "/health":
			p.write(w, http.StatusOK, map[string]any{"status": "ok", "version": p.Version})
		case "/ready":
			p.Ready.ServeHTTP(w, r)
		default:
			next.ServeHTTP(w, r)
		}
	})
}

func (p Probes) write(w http.ResponseWriter, status int, body map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(body); err != nil {
		p.Log.Warn("write probe response", "err", err)
	}
}
