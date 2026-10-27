package catalog

import (
	"errors"
	"fmt"

	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/config"
	"github.com/AlexGalhardo/vukem/drivers/mongo"
	"github.com/AlexGalhardo/vukem/drivers/s3"
	"github.com/AlexGalhardo/vukem/storage"
)

// DiskConfig picks where images live (MEDIA_DISK): "vercel-blob", "s3",
// "local" or "memory". Empty means vercel-blob when BLOB_READ_WRITE_TOKEN is
// set, s3 when S3_BUCKET is set, otherwise memory in tests and the local
// directory in development; production requires a remote disk (blob or s3).
type DiskConfig struct {
	Driver   string `env:"MEDIA_DISK"`
	Bucket   string `env:"S3_BUCKET"`
	Blob     string `env:"BLOB_READ_WRITE_TOKEN"`
	LocalDir string `env:"MEDIA_LOCAL_DIR" default:"storage/app/media"`
}

var errDiskRequired = errors.New("catalog: BLOB_READ_WRITE_TOKEN or S3_BUCKET is required in production (MEDIA_DISK=vercel-blob or s3)")

// OpenDisk returns the image disk configured in src.
func OpenDisk(src config.Lookuper, env config.Env, test bool) (storage.Disk, error) {
	cfg, err := config.Load[DiskConfig](src)
	if err != nil {
		return nil, fmt.Errorf("catalog: media config: %w", err)
	}

	driver := cfg.Driver
	if driver == "" {
		switch {
		case test:
			// Tests never reach a real bucket by accident (.env may set one).
			driver = "memory"
		case cfg.Blob != "":
			driver = "vercel-blob"
		case cfg.Bucket != "":
			driver = "s3"
		default:
			driver = "local"
		}
	}

	if env == config.Production && driver != "s3" && driver != "vercel-blob" {
		return nil, errDiskRequired
	}

	switch driver {
	case "s3":
		s3cfg, err := config.Load[s3.Config](src)
		if err != nil {
			return nil, fmt.Errorf("catalog: s3 config: %w", err)
		}

		return s3.New(s3cfg)
	case "vercel-blob":
		bcfg, err := config.Load[BlobConfig](src)
		if err != nil {
			return nil, fmt.Errorf("catalog: blob config: %w", err)
		}

		return NewVercelBlob(bcfg)
	case "local":
		return storage.NewLocal(cfg.LocalDir, "")
	case "memory":
		return storage.NewMemory(""), nil
	default:
		return nil, fmt.Errorf("catalog: unknown MEDIA_DISK %q (vercel-blob, s3, local or memory)", driver)
	}
}

// FromApp builds the service from the app: its database, cache (Redis when
// configured), the configured image disk and, when mongoClient is not nil,
// MongoDB history. Other modules (cart, checkout, APIs) call it too; the
// service is stateless apart from the memory disk used in tests.
func FromApp(app *vukem.App, mongoClient *mongo.Client) (*Service, error) {
	db, err := app.DB()
	if err != nil {
		return nil, err
	}

	c, err := app.Cache()
	if err != nil {
		return nil, err
	}

	disk, err := OpenDisk(app.Source, app.Config.Env, app.Kind == vukem.KindTest)
	if err != nil {
		return nil, err
	}

	q, err := app.Queue()
	if err != nil {
		return nil, err
	}

	cfg := Config{DB: db, Cache: c, Disk: disk, Queue: q, Log: app.Log, Now: app.Now}
	if mongoClient != nil {
		cfg.Recorder = NewMongoRecorder(mongoClient)
	}

	return New(cfg)
}
