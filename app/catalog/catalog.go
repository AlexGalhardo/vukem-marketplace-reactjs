// Package catalog is the catalog service: categories, shops, products,
// product images and search. Controllers, the cart/checkout stream and the
// REST/GraphQL/gRPC APIs all call it, so every visibility and ownership
// rule lives here and nowhere else:
//
//   - a product is public only when it is published and its shop is active
//     (shop payability for card checkout is the checkout stream's concern);
//   - seller methods take the acting user id and answer ErrNotFound for
//     anything that user does not own (never 403, so ids do not leak).
package catalog

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/AlexGalhardo/vukem/cache"
	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/errs"
	"github.com/AlexGalhardo/vukem/queue"
	"github.com/AlexGalhardo/vukem/storage"
)

var (
	// ErrNotFound: the resource does not exist, is not public or is not
	// owned by the acting user (404).
	ErrNotFound = errs.New(http.StatusNotFound, "Not found.")
	// ErrShopExists: the user already has a shop (409).
	ErrShopExists = errs.New(http.StatusConflict, "You already have a shop.")
	// ErrNoShop: the acting user has no shop yet.
	ErrNoShop = errors.New("catalog: user has no shop")
	// ErrTooManyImages: the product already has MaxImages images (422).
	ErrTooManyImages = errs.New(http.StatusUnprocessableEntity, "A product can have at most 8 images.")
)

// Recorder stores search and product view history (MongoDB). Failures are
// logged by the caller and never fail a page: the history is non-critical.
type Recorder interface {
	RecordSearch(ctx context.Context, ev SearchEvent) error
	RecordView(ctx context.Context, ev ViewEvent) error
}

// Config holds the service dependencies.
type Config struct {
	DB *database.DB
	// Cache stores anonymous search pages and the category list (Redis in
	// production). Nil disables caching.
	Cache *cache.Cache
	// Disk stores product and shop images (S3 in production).
	Disk storage.Disk
	// Queue runs ThumbnailJob; nil makes thumbnails inline (tools, tests).
	Queue *queue.Queue
	// Recorder stores search/view history; nil discards it.
	Recorder Recorder
	Log      *slog.Logger
	Now      func() time.Time
}

// Service is the catalog API used by controllers and other modules.
type Service struct {
	db       *database.DB
	cache    *cache.Cache
	disk     storage.Disk
	queue    *queue.Queue
	recorder Recorder
	log      *slog.Logger
	now      func() time.Time
}

var errNoDB = errors.New("catalog: Config.DB and Config.Disk are required")

// New returns the service.
func New(cfg Config) (*Service, error) {
	if cfg.DB == nil || cfg.Disk == nil {
		return nil, errNoDB
	}

	s := &Service{
		db:       cfg.DB,
		cache:    cfg.Cache,
		disk:     cfg.Disk,
		queue:    cfg.Queue,
		recorder: cfg.Recorder,
		log:      cfg.Log,
		now:      cfg.Now,
	}
	if s.log == nil {
		s.log = slog.New(slog.DiscardHandler)
	}

	if s.now == nil {
		s.now = time.Now
	}

	return s, nil
}

// Disk returns the image disk (the media route serves from it).
func (s *Service) Disk() storage.Disk { return s.disk }

// isPostgres reports whether the real full-text search SQL can run.
func (s *Service) isPostgres() bool { return s.db.Dialect().Name() == "postgres" }

// invalid is a validation error on one field (422 for APIs, errors bag
// for Inertia forms).
func invalid(field, message string) error {
	return errs.NewValidation().Add(field, message)
}
