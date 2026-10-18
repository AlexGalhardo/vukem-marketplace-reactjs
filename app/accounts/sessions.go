package accounts

import (
	"github.com/AlexGalhardo/vukem/auth"
	"github.com/AlexGalhardo/vukem/web"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/models"
)

// Login logs u in with the web guard (new session id). The guard records
// u's session version, so revokeCredentials ends this session later.
func (a *Accounts) Login(c *web.Ctx, u models.User, opts ...auth.LoginOption) error {
	return a.Web.Login(c, u, opts...)
}
