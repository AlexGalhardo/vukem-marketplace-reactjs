package chat

import (
	"context"
	"errors"
	"strconv"
	"sync"

	"github.com/AlexGalhardo/vukem/auth"
	"github.com/AlexGalhardo/vukem/broadcast"
	"github.com/AlexGalhardo/vukem/sse"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/models"
)

// MemberInfo is what the presence channel shows about a participant.
type MemberInfo struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

// NewBroadcaster returns the broadcast server of the chat: the private
// message channel and the presence channel of each conversation (its two
// participants only) and each user's private inbox channel. broker fans
// events out to every instance (nil: this process only).
//
// workaround(vukem): broadcast/sse publish on Redis channels named
// "sse:broadcast:<channel>" with no app prefix, and Redis pub/sub ignores
// the database number, so two apps (or environments) on one Redis would
// receive each other's events; prefixBroker namespaces them.
func NewBroadcaster(broker sse.Broker, prefix string, svc *Service) (*broadcast.Server, error) {
	if broker != nil && prefix != "" {
		broker = prefixBroker{inner: broker, prefix: prefix}
	}

	b := broadcast.New(broadcast.Config{
		Broker: broker,
		User: func(ctx context.Context) (any, error) {
			u, err := auth.UserFrom[models.User](ctx, "web")
			if err != nil {
				return nil, err
			}

			return &u, nil
		},
	})

	participant := func(ctx context.Context, user any, params map[string]string) (*models.User, string, error) {
		u, ok := user.(*models.User)
		if !ok {
			return nil, "", errors.New("chat: unexpected user type")
		}

		id, perr := strconv.ParseInt(params["id"], 10, 64)
		if perr != nil || id <= 0 {
			return nil, "", nil //nolint:nilerr // not a conversation id: deny without an error
		}

		_, role, err := svc.Participant(ctx, u.ID, id)
		if errors.Is(err, ErrNotFound) {
			return nil, "", nil
		}

		return u, role, err
	}

	err := errors.Join(
		b.Private("chat.{id}", func(ctx context.Context, user any, p map[string]string) (bool, error) {
			u, _, err := participant(ctx, user, p)
			return u != nil, err
		}),
		b.Presence(
			"chat.{id}",
			func(ctx context.Context, user any, p map[string]string) (broadcast.Member, bool, error) {
				u, role, err := participant(ctx, user, p)
				if u == nil || err != nil {
					return broadcast.Member{}, false, err
				}

				info := MemberInfo{Name: FirstName(u.Name), Role: role}

				return broadcast.Member{ID: strconv.FormatInt(u.ID, 10), Info: info}, true, nil
			},
		),
		b.Private("inbox.{id}", func(_ context.Context, user any, p map[string]string) (bool, error) {
			u, ok := user.(*models.User)
			return ok && p["id"] == strconv.FormatInt(u.ID, 10), nil
		}),
	)
	if err != nil {
		b.Close()
		return nil, err
	}

	return b, nil
}

// prefixBroker namespaces the Redis pub/sub channels of a broker.
type prefixBroker struct {
	inner  sse.Broker
	prefix string
}

func (p prefixBroker) Publish(ctx context.Context, channel string, payload []byte) error {
	return p.inner.Publish(ctx, p.prefix+channel, payload)
}

func (p prefixBroker) Subscribe(ctx context.Context, channels ...string) (<-chan []byte, error) {
	out := make([]string, len(channels))
	for i, c := range channels {
		out[i] = p.prefix + c
	}

	return p.inner.Subscribe(ctx, out...)
}

// Connections caps the open sockets (and SSE streams) of each user on this
// instance.
//
// shortcut: the count is per instance, so a user may hold Max sockets on
// each instance; move it to a shared counter (Redis INCR with a TTL) if
// that is ever abused.
type Connections struct {
	Max int

	mu   sync.Mutex
	open map[int64]int
}

// Acquire reserves a connection for userID; false when they are at Max.
func (c *Connections) Acquire(userID int64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.open == nil {
		c.open = map[int64]int{}
	}

	if c.open[userID] >= c.Max {
		return false
	}

	c.open[userID]++

	return true
}

// Release frees a connection Acquire reserved.
func (c *Connections) Release(userID int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.open[userID]--
	if c.open[userID] <= 0 {
		delete(c.open, userID)
	}
}
