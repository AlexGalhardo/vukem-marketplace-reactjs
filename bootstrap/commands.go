package bootstrap

import (
	"context"
	"errors"
	"flag"

	vukem "github.com/AlexGalhardo/vukem"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/accounts"
)

// Commands lists the application commands; vukem make:command appends here.
func Commands() []vukem.Command {
	return []vukem.Command{makeAdmin{}}
}

var errEmailRequired = errors.New("usage: users:make-admin <email>")

// makeAdmin promotes an existing account to admin. It is the only way to
// create admins (no web or API path); the admin must then enable 2FA.
type makeAdmin struct{}

func (makeAdmin) Name() string { return "users:make-admin" }

func (makeAdmin) Description() string { return "Grant the admin role to an existing user by email" }

func (makeAdmin) Flags(*flag.FlagSet) {}

func (makeAdmin) Run(ctx context.Context, app *vukem.App, args []string) error {
	if len(args) != 1 {
		return errEmailRequired
	}

	acc, err := accounts.New(app)
	if err != nil {
		return err
	}

	if err := acc.MakeAdmin(ctx, args[0]); err != nil {
		return err
	}

	// workaround(vukem): commands cannot reach the app's stdout writer
	// (WithStdout is private state), so the confirmation goes to the log.
	app.Log.InfoContext(ctx, "user is now an admin; 2FA is required before using /admin", "email", args[0])

	return nil
}
