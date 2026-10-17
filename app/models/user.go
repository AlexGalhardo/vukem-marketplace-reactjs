package models

import (
	"strconv"

	"github.com/AlexGalhardo/vukem/orm"
)

// User is a registered account (vukem install:auth).
type User struct {
	orm.Model

	Name            string       `json:"name"`
	Email           string       `json:"email"`
	Password        string       `json:"-"`
	EmailVerifiedAt orm.NullTime `json:"email_verified_at"`
	Role            string       `json:"role"`
	// SessionVersion ends every browser session when it changes
	// (auth.SessionVersioner).
	SessionVersion int64 `json:"-"`
}

// Roles.
const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

// IsAdmin reports whether u may use the admin area (2FA is enforced
// separately by accounts.RequireAdmin).
func (u User) IsAdmin() bool { return u.Role == RoleAdmin }

// AuthID implements auth.Authenticatable.
func (u User) AuthID() string { return strconv.FormatInt(u.ID, 10) }

// AuthSessionVersion implements auth.SessionVersioner.
func (u User) AuthSessionVersion() int64 { return u.SessionVersion }

// AuthPassword implements auth.Authenticatable.
func (u User) AuthPassword() string { return u.Password }

// EmailVerified implements auth.Verifiable.
func (u User) EmailVerified() bool { return u.EmailVerifiedAt.Valid }
