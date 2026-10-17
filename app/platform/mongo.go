package platform

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/x/mongo/driver/connstring"

	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/drivers/mongo"
)

var errMongoRequired = errors.New("platform: MONGO_URL is required in production")

// DefaultMongoDatabase is used when neither MONGO_DATABASE nor the
// MONGO_URL path names a database.
const DefaultMongoDatabase = "marketplace"

// MongoDatabase picks the database name: MONGO_DATABASE, else the URI
// path, else DefaultMongoDatabase. Railway's MongoDB plugin URL has no
// path, and appending one also changes the default authSource from admin
// to that database, so MONGO_DATABASE keeps the plugin URL usable as is.
func MongoDatabase(uri, configured string) (string, error) {
	if configured != "" {
		return configured, nil
	}

	cs, err := connstring.ParseAndValidate(uri)
	if err != nil {
		return "", fmt.Errorf("platform: parse MONGO_URL: %w", err)
	}

	if cs.Database != "" {
		return cs.Database, nil
	}

	return DefaultMongoDatabase, nil
}

// MongoProvider connects MongoDB at boot when MONGO_URL is set
// (mongodb://host:27017/marketplace, or a URL without a path plus
// MONGO_DATABASE). It reads the app's configuration source, so tests can
// point it at a throwaway database.
type MongoProvider struct {
	Deps *Deps
}

// Register implements vukem.Provider; connecting happens in Boot.
func (p MongoProvider) Register(*vukem.App) error { return nil }

// Boot implements vukem.Booter.
func (p MongoProvider) Boot(_ context.Context, app *vukem.App) error {
	url := app.Source.Get("MONGO_URL")
	if url == "" {
		if app.Config.IsProduction() {
			return errMongoRequired
		}

		return nil
	}

	name, err := MongoDatabase(url, app.Source.Get("MONGO_DATABASE"))
	if err != nil {
		return err
	}

	// Lazy: the app boots while MongoDB is down; /ready reports it.
	mc, err := mongo.OpenLazy(url, name)
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
