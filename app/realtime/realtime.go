// Package realtime pushes per-user events to browsers over server-sent
// events (GET /events). Publishers on any instance (web, worker,
// scheduler) send through Redis pub/sub; every web instance relays them to
// its local streams. Each user only ever subscribes to their own topics,
// orders:<userID> and notifications:<userID>.
//
// Checkout calls Hub.Publish on order status changes:
//
//	deps.Realtime.Publish(ctx, buyerID, realtime.Orders, map[string]any{"order": publicID, "status": "paid"})
package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/AlexGalhardo/vukem/sse"
)

// Channels a user can receive; the SSE event type is the channel name.
const (
	Orders        = "orders"
	Notifications = "notifications"
)

// Channels lists every channel a stream subscribes to.
func Channels() []string { return []string{Orders, Notifications} }

// ErrUnknownChannel is returned by Publish for a channel outside Channels.
var ErrUnknownChannel = errors.New("realtime: unknown channel")

// Broker is the Redis pub/sub client (*redis.Client from drivers/redis).
type Broker interface {
	Publish(ctx context.Context, channel string, payload []byte) error
	Subscribe(ctx context.Context, channels ...string) (<-chan []byte, error)
}

// Hub publishes events and serves streams.
//
// workaround(vukem): sse.Hub with a Broker starts one Redis subscription
// and goroutine per topic for the life of the hub and never releases them,
// so per-user topics would leak a subscription per user ever connected.
// Here all events share one Redis channel and a single relay feeds a
// broker-less sse.Hub.
type Hub struct {
	local   *sse.Hub
	broker  Broker
	channel string
	log     *slog.Logger
}

// NewHub returns a hub. With a nil broker (unit tests, no Redis) events
// only reach streams of this process.
func NewHub(broker Broker, redisChannel string, log *slog.Logger) *Hub {
	if log == nil {
		log = slog.Default()
	}

	return &Hub{local: sse.NewHub(nil), broker: broker, channel: redisChannel, log: log}
}

// Topic is the hub topic of a user's channel, e.g. "orders:42".
func Topic(channel string, userID int64) string {
	return channel + ":" + strconv.FormatInt(userID, 10)
}

// envelope is what travels over Redis.
type envelope struct {
	Topic string    `json:"topic"`
	Event sse.Event `json:"event"`
}

// Publish sends payload (JSON-encoded) to userID's streams on channel.
// Delivery is best effort: a user without an open stream misses it, so
// pages must also load the current state on their own.
func (h *Hub) Publish(ctx context.Context, userID int64, channel string, payload any) error {
	if channel != Orders && channel != Notifications {
		return fmt.Errorf("%w %q", ErrUnknownChannel, channel)
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("realtime: encode payload: %w", err)
	}

	env := envelope{Topic: Topic(channel, userID), Event: sse.Event{Event: channel, Data: string(data)}}

	if h.broker == nil {
		return h.local.Publish(ctx, env.Topic, env.Event)
	}

	b, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("realtime: encode event: %w", err)
	}

	if err := h.broker.Publish(ctx, h.channel, b); err != nil {
		return fmt.Errorf("realtime: publish: %w", err)
	}

	return nil
}

// Stream sends userID's events to s until the client leaves.
func (h *Hub) Stream(s *sse.Stream, userID int64) error {
	return h.local.Stream(s, topics(userID)...)
}

// Subscribe returns userID's events on this instance (for streams other
// than SSE, such as a gRPC watch) and the function that unsubscribes.
func (h *Hub) Subscribe(ctx context.Context, userID int64) (<-chan sse.Event, func()) {
	return h.local.Subscribe(ctx, topics(userID)...)
}

func topics(userID int64) []string {
	out := make([]string, 0, len(Channels()))
	for _, c := range Channels() {
		out = append(out, Topic(c, userID))
	}

	return out
}

// Run relays Redis messages to the local streams until ctx is done; it is
// the app runner of every web instance. Without a broker it just waits.
func (h *Hub) Run(ctx context.Context) error {
	if h.broker == nil {
		<-ctx.Done()
		return nil
	}

	msgs, err := h.broker.Subscribe(ctx, h.channel)
	if err != nil {
		return fmt.Errorf("realtime: subscribe: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case b, ok := <-msgs:
			if !ok {
				return nil
			}

			var env envelope
			if err := json.Unmarshal(b, &env); err != nil {
				h.log.WarnContext(ctx, "realtime: dropped malformed event", "err", err)
				continue
			}

			_ = h.local.Publish(ctx, env.Topic, env.Event) // broker-less: never fails
		}
	}
}

// Close ends every open stream (shutdown).
func (h *Hub) Close() { h.local.Close() }
