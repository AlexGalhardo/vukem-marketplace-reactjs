// Package admin holds the admin area handlers; every route is behind
// accounts.RequireAdmin.
package admin

import (
	"errors"

	"github.com/AlexGalhardo/vukem/errs"
	"github.com/AlexGalhardo/vukem/i18n"
	"github.com/AlexGalhardo/vukem/inertia"
	"github.com/AlexGalhardo/vukem/web"
	"github.com/AlexGalhardo/vukem/z"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/accounts"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/flags"
)

// FlagForm switches one flag.
type FlagForm struct {
	Flag    string `json:"flag"    form:"flag"`
	Enabled bool   `json:"enabled" form:"enabled"`
}

// FlagFormSchema validates FlagForm.
var FlagFormSchema = z.Struct[FlagForm](z.Shape{
	"flag":    z.String().Trim().Min(1).Max(64),
	"enabled": z.Bool(),
})

// historySize is how many changes the page shows.
const historySize = 50

// Flags serves /admin/flags.
type Flags struct {
	Accounts *accounts.Accounts
	Flags    *flags.Flags
}

type flagRow struct {
	flags.Definition

	Enabled bool `json:"enabled"`
}

// Show lists the flags with their values and the latest changes.
func (h *Flags) Show(c *web.Ctx) error {
	values := h.Flags.All(c.Context())

	rows := []flagRow{}
	for _, d := range flags.Definitions() {
		rows = append(rows, flagRow{Definition: d, Enabled: values[d.Key]})
	}

	changes, err := h.Flags.Changes(c.Context(), historySize)
	if err != nil {
		return err
	}

	return inertia.Render(c, "Admin/Flags", inertia.Props{"flags": rows, "changes": changes})
}

// Update switches a flag; locked flags are refused (403).
func (h *Flags) Update(c *web.Ctx) error {
	u, err := h.Accounts.Web.User(c)
	if err != nil {
		return errs.ErrUnauthorized
	}

	in, err := web.Parse(c, FlagFormSchema)
	if err != nil {
		return err
	}

	err = h.Flags.Set(c.Context(), flags.Key(in.Flag), in.Enabled, flags.Actor{ID: u.ID, Email: u.Email})

	switch {
	case errors.Is(err, flags.ErrUnknown):
		return errs.ErrNotFound
	case errors.Is(err, flags.ErrLocked):
		return errs.New(403, i18n.T(c.Context(), "admin.flags.locked_error"))
	case err != nil:
		return err
	}

	web.SecurityEvent(c.Request(), "admin.flag_changed", "flag", in.Flag, "enabled", in.Enabled, "admin", u.ID)
	inertia.Flash(c, "success", i18n.T(c.Context(), "admin.flags.saved"))

	return c.Redirect("/admin/flags")
}
