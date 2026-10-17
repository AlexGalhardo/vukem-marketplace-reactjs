package platform

import (
	"context"
	"errors"
	"fmt"
	"time"

	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/drivers/mongo"
)

var errMongoRequired = errors.New("platform: MONGO_URL is required in production")

// MongoProvider connects MongoDB at boot when MONGO_URL is set
// (mongodb://host:27017/marketplace). It reads the app's configuration
// source, so tests can point it at a throwaway database.
type MongoProvider struct {
	Deps *Deps
}

// Register implements vukem.Provider; connecting happens in Boot.
func (p MongoProvider) Register(*vukem.App) error { return nil }

// Boot implements vukem.Booter.
func (p MongoProvider) Boot(ctx context.Context, app *vukem.App) error {
	url := app.Source.Get("MONGO_URL")
	if url == "" {
		if app.Config.IsProduction() {
			return errMongoRequired
		}

		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	mc, err := mongo.Open(ctx, url)
	if err != nil {
		return fmt.Errorf("platform: open mongo: %w", err)
	}

	p.Deps.Mongo = mc
	p.Deps.AddCheck("mongo", mc.Ping)

	return nil
}

// Shutdown implements vukem.Shutdowner.
func (p MongoProvider) Shutdown(ctx context.Context, _ *vukem.App) error {
	if p.Deps.Mongo == nil {
		return nil
	}

	return p.Deps.Mongo.Close(ctx)
}
