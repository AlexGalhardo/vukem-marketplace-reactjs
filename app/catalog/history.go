package catalog

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	mongodrv "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/AlexGalhardo/vukem/drivers/mongo"
)

// historyTTL is how long search and view history is kept (PLAN 3.5).
const historyTTL = 90 * 24 * time.Hour

// SearchEvent is one search, built only from validated Query fields.
type SearchEvent struct {
	Q        string    `bson:"q"`
	Category string    `bson:"category,omitempty"`
	Shop     string    `bson:"shop,omitempty"`
	MinCents int64     `bson:"min_cents,omitempty"`
	MaxCents int64     `bson:"max_cents,omitempty"`
	UF       string    `bson:"uf,omitempty"`
	Sort     string    `bson:"sort"`
	Results  int64     `bson:"results"`
	UserID   int64     `bson:"user_id,omitempty"`
	AnonID   string    `bson:"anon_id,omitempty"`
	At       time.Time `bson:"at"`
}

// ViewEvent is one product page view.
type ViewEvent struct {
	ProductID int64     `bson:"product_id"`
	ShopID    int64     `bson:"shop_id"`
	UserID    int64     `bson:"user_id,omitempty"`
	AnonID    string    `bson:"anon_id,omitempty"`
	At        time.Time `bson:"at"`
}

// MongoRecorder stores history in the search_history and product_views
// collections. Documents are typed structs, never request maps, so no
// client value can become a query operator.
//
// The client is read on each call: MongoDB connects in a provider's Boot,
// after routes (and so this recorder) are built. Until it is connected,
// or when MONGO_URL is unset, events are dropped.
type MongoRecorder struct {
	client  func() *mongo.Client
	indexed atomic.Bool
}

// NewMongoRecorder returns a recorder over the client client returns.
func NewMongoRecorder(client func() *mongo.Client) *MongoRecorder {
	return &MongoRecorder{client: client}
}

// EnsureIndexes creates the TTL indexes (idempotent).
func EnsureIndexes(ctx context.Context, mc *mongo.Client) error {
	ttl := mongodrv.IndexModel{
		Keys:    bson.D{{Key: "at", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(int32(historyTTL / time.Second)),
	}

	for _, name := range []string{"search_history", "product_views"} {
		if _, err := mc.Database().Collection(name).Indexes().CreateOne(ctx, ttl); err != nil {
			return fmt.Errorf("catalog: ttl index on %s: %w", name, err)
		}
	}

	_, err := mc.Database().Collection("product_views").Indexes().
		CreateOne(ctx, mongodrv.IndexModel{Keys: bson.D{{Key: "product_id", Value: 1}}})
	if err != nil {
		return fmt.Errorf("catalog: index on product_views: %w", err)
	}

	return nil
}

// ready returns the client once connected, creating the indexes on first
// use (retried on the next event if that fails).
func (m *MongoRecorder) ready(ctx context.Context) (*mongo.Client, error) {
	mc := m.client()
	if mc == nil {
		return nil, nil
	}

	if !m.indexed.Load() {
		if err := EnsureIndexes(ctx, mc); err != nil {
			return nil, err
		}

		m.indexed.Store(true)
	}

	return mc, nil
}

// RecordSearch implements Recorder.
func (m *MongoRecorder) RecordSearch(ctx context.Context, ev SearchEvent) error {
	mc, err := m.ready(ctx)
	if mc == nil {
		return err
	}

	_, err = mongo.Collection[SearchEvent](mc, "search_history").Insert(ctx, ev)

	return err
}

// RecordView implements Recorder.
func (m *MongoRecorder) RecordView(ctx context.Context, ev ViewEvent) error {
	mc, err := m.ready(ctx)
	if mc == nil {
		return err
	}

	_, err = mongo.Collection[ViewEvent](mc, "product_views").Insert(ctx, ev)

	return err
}

// historyTimeout keeps a slow Mongo from slowing pages down.
const historyTimeout = time.Second

// RecordSearch stores a search (page 1 with a term only); failures are
// logged and swallowed.
func (s *Service) RecordSearch(ctx context.Context, q Query, results, userID int64, anonID string) {
	q = q.Normalized()
	if s.recorder == nil || q.Q == "" || q.Page != 1 {
		return
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), historyTimeout)
	defer cancel()

	err := s.recorder.RecordSearch(ctx, SearchEvent{
		Q: q.Q, Category: q.Category, Shop: q.Shop, MinCents: q.MinCents, MaxCents: q.MaxCents, UF: q.UF,
		Sort: q.Sort, Results: results, UserID: userID, AnonID: anonID, At: s.now().UTC(),
	})
	if err != nil {
		s.log.WarnContext(ctx, "catalog: record search", "error", err)
	}
}

// RecordView stores a product view; failures are logged and swallowed.
func (s *Service) RecordView(ctx context.Context, p ProductDetail, userID int64, anonID string) {
	if s.recorder == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), historyTimeout)
	defer cancel()

	err := s.recorder.RecordView(ctx, ViewEvent{
		ProductID: p.ID, ShopID: p.Shop.ID, UserID: userID, AnonID: anonID, At: s.now().UTC(),
	})
	if err != nil {
		s.log.WarnContext(ctx, "catalog: record view", "error", err)
	}
}
