// Package apikeys renders the account page where users create, list and
// revoke their API keys (/account/api-keys).
package apikeys

import (
	"errors"
	"slices"
	"strconv"
	"time"

	"github.com/AlexGalhardo/vukem/auth"
	"github.com/AlexGalhardo/vukem/errs"
	"github.com/AlexGalhardo/vukem/inertia"
	"github.com/AlexGalhardo/vukem/web"
	"github.com/AlexGalhardo/vukem/z"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/api"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/models"
)

// Path of the page.
const Path = "/account/api-keys"

// createdFlash carries the new token to the next page view only.
const createdFlash = "api_key_created"

// Expiry choices, in days; 0 means the key never expires.
var expiryDays = []int{0, 30, 90, 365} //nolint:gochecknoglobals // read-only choices

// CreateInput is the create form.
type CreateInput struct {
	Name          string   `json:"name"            form:"name"            validate:"required,max=60"`
	Scopes        []string `json:"scopes"          form:"scopes"          validate:"required,min=1,max=6"`
	ExpiresInDays int      `json:"expires_in_days" form:"expires_in_days" validate:"min=0,max=365"`
}

// Handlers serve the page.
type Handlers struct {
	Keys *api.Keys
	Now  func() time.Time
}

func userID(c *web.Ctx) (int64, error) {
	u, err := auth.User[models.User](c, "web")
	if err != nil {
		return 0, err
	}

	return u.ID, nil
}

// Show lists the user's keys; right after creation it shows the token once.
func (h *Handlers) Show(c *web.Ctx) error {
	id, err := userID(c)
	if err != nil {
		return err
	}

	keys, err := h.Keys.List(c.Context(), id)
	if err != nil {
		return err
	}

	now := h.Now()
	rows := make([]map[string]any, 0, len(keys))

	for i := range keys {
		k := &keys[i]
		rows = append(rows, map[string]any{
			"id": k.ID, "name": k.Name, "prefix": api.TokenPrefix + k.Prefix, "scopes": k.Scopes,
			"last_used_at": k.LastUsedAt, "expires_at": k.ExpiresAt, "revoked_at": k.RevokedAt,
			"created_at": k.CreatedAt, "active": k.Active(now),
		})
	}

	return inertia.Render(c, "Account/ApiKeys", inertia.Props{
		"keys":       rows,
		"scopes":     api.AllScopes(),
		"expiryDays": expiryDays,
		"maxKeys":    api.MaxActiveKeys,
		"created":    c.Session().Get(createdFlash),
	})
}

// Store creates a key and redirects to Show, which displays it once.
func (h *Handlers) Store(c *web.Ctx) error {
	id, err := userID(c)
	if err != nil {
		return err
	}

	in, err := web.Parse(c, z.For[CreateInput]())
	if err != nil {
		return err
	}

	if !slices.Contains(expiryDays, in.ExpiresInDays) {
		return errs.NewValidation().Add("expires_in_days", "Choose one of the listed expirations.")
	}

	nk := api.NewKey{Name: in.Name, Scopes: in.Scopes}
	if in.ExpiresInDays > 0 {
		nk.ExpiresAt = new(h.Now().UTC().AddDate(0, 0, in.ExpiresInDays))
	}

	key, token, err := h.Keys.Create(c.Context(), id, nk)

	switch {
	case errors.Is(err, api.ErrTooManyKeys):
		return errs.NewValidation().Add("name", "You already have the maximum number of active API keys.")
	case errors.Is(err, api.ErrInvalidInput):
		return api.HTTPError(err)
	case err != nil:
		return err
	}

	web.SecurityEvent(c.Request(), "api.key_created", "user_id", strconv.FormatInt(id, 10),
		"key_id", strconv.FormatInt(key.ID, 10))
	c.Session().Flash(createdFlash, map[string]any{"name": key.Name, "token": token})

	return c.Redirect(Path)
}

// Destroy revokes one of the user's keys; anyone else's is a 404.
func (h *Handlers) Destroy(c *web.Ctx) error {
	id, err := userID(c)
	if err != nil {
		return err
	}

	keyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return errs.ErrNotFound
	}

	if err := h.Keys.Revoke(c.Context(), id, keyID); err != nil {
		return api.HTTPError(err)
	}

	web.SecurityEvent(c.Request(), "api.key_revoked", "user_id", strconv.FormatInt(id, 10),
		"key_id", strconv.FormatInt(keyID, 10))

	return c.Redirect(Path)
}
