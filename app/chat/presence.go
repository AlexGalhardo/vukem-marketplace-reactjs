package chat

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/AlexGalhardo/vukem/cache"
)

// Last-seen tracking. Every chat request, every page a signed-in user
// renders and every open socket (each HeartbeatEvery) touch the user's
// last-seen time in the shared cache, so any instance can tell whether
// someone is online and how long they have been away.
//
// It complements the presence channel: broadcast presence is per instance
// (its "here" list only shows members connected to the same instance), so
// the initial online state of the other participant comes from here.
const (
	// OnlineWindow: seen this recently counts as online.
	OnlineWindow = 75 * time.Second
	// HeartbeatEvery is how often an open socket refreshes last-seen.
	HeartbeatEvery = 30 * time.Second
	seenTTL        = 30 * 24 * time.Hour
)

func seenKey(userID int64) string { return "chat:seen:" + strconv.FormatInt(userID, 10) }

// Touch records that userID is around now. Failures are logged: presence
// is a hint, never a reason to fail a request.
func (s *Service) Touch(ctx context.Context, userID int64) {
	if err := s.cache.Put(ctx, seenKey(userID), s.now().Unix(), seenTTL); err != nil {
		s.log.WarnContext(ctx, "chat: last-seen not stored", "err", err, "user_id", userID)
	}
}

// LastSeen returns when userID was last around (ok false: never, or more
// than 30 days ago).
func (s *Service) LastSeen(ctx context.Context, userID int64) (time.Time, bool, error) {
	v, ok, err := cache.Get[int64](ctx, s.cache, seenKey(userID))
	if err != nil {
		return time.Time{}, false, fmt.Errorf("chat: last-seen of user %d: %w", userID, err)
	}

	if !ok {
		return time.Time{}, false, nil
	}

	return time.Unix(v, 0), true, nil
}

// Online reports whether userID was seen within OnlineWindow (false when
// the cache cannot tell).
func (s *Service) Online(ctx context.Context, userID int64) bool {
	at, ok, err := s.LastSeen(ctx, userID)

	return err == nil && ok && s.now().Sub(at) < OnlineWindow
}

// KeepSeen touches userID now and every HeartbeatEvery until ctx ends
// (an open socket), then once more as it closes.
func (s *Service) KeepSeen(ctx context.Context, userID int64) {
	s.Touch(ctx, userID)

	t := time.NewTicker(HeartbeatEvery)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			s.Touch(context.WithoutCancel(ctx), userID)
			return
		case <-t.C:
			s.Touch(ctx, userID)
		}
	}
}
