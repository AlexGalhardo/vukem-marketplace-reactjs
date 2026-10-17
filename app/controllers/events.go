package controllers

import (
	"github.com/AlexGalhardo/vukem/auth"
	"github.com/AlexGalhardo/vukem/sse"
	"github.com/AlexGalhardo/vukem/web"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/models"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/realtime"
)

// Events streams the signed-in user's realtime events (GET /events). The
// topics come from the session user only, never from the request, so a
// client cannot subscribe to someone else's channel.
type Events struct {
	Hub *realtime.Hub
}

// Stream serves the event stream until the client leaves.
func (h Events) Stream(c *web.Ctx) error {
	u, err := auth.User[models.User](c, "web")
	if err != nil {
		return err
	}

	return c.SSE(func(s *sse.Stream) error { return h.Hub.Stream(s, u.ID) })
}
