package controllers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/AlexGalhardo/vukem/errs"
	"github.com/AlexGalhardo/vukem/inertia"
	"github.com/AlexGalhardo/vukem/web"
)

// ErrorPages renders handler errors as the Inertia "Error" page for
// browser and Inertia visits, so an Inertia client shows the app's page
// instead of its raw-HTML modal. JSON and htmx clients, validation errors
// (flashed back to the form) and requests outside the Inertia middleware
// keep vukem's default handling.
//
// workaround(vukem): web.DefaultErrorHandler answers Inertia visits with
// a raw HTML status page.
func ErrorPages(c *web.Ctx, err error) {
	status := errs.Status(err)

	_, isValidation := errors.AsType[*errs.ValidationError](err)
	if c.WantsJSON() || c.IsHTMX() || isValidation || status == http.StatusUnprocessableEntity || status < 400 {
		web.DefaultErrorHandler(c, err)
		return
	}

	level := slog.LevelInfo
	if status >= 500 {
		level = slog.LevelError
	}

	c.Log().Log(c.Context(), level, "request error",
		"status", status, "method", c.Request().Method, "path", c.Request().URL.Path, "err", err,
		"request_id", web.RequestID(c.Context()))

	// 5xx details never reach the client.
	message := ""
	if he, ok := errors.AsType[*errs.HTTPError](err); ok && status < 500 {
		message = he.LocalizedText(c.Context())
	}

	props := inertia.Props{"status": status, "message": message}
	if rerr := inertia.RenderStatus(c, status, "Error", props); rerr != nil {
		web.DefaultErrorHandler(c, err)
	}
}
