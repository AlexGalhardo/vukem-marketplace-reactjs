package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/AlexGalhardo/vukem/web"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/requests"
)

// ClientLog receives browser error reports and hands them to the
// client-logger service (services/client-logger, built on telegram-logger),
// so the Telegram token never reaches the browser. With no URL the
// endpoint accepts and drops reports.
type ClientLog struct {
	URL    string // CLIENT_LOGGER_URL, e.g. http://client-logger:3100
	Key    string // CLIENT_LOGGER_KEY, shared secret of the service
	Client *http.Client
	Log    *slog.Logger
}

// Store validates a report and forwards it; the browser always gets 204
// once the report is valid, even if the forward fails.
func (h ClientLog) Store(c *web.Ctx) error {
	in, err := web.Parse(c, requests.ClientLogSchema)
	if err != nil {
		return err
	}

	if h.URL != "" {
		// Detached from the request: the browser does not wait for Telegram.
		go h.forward(context.WithoutCancel(c.Context()), in)
	}

	return c.NoContent()
}

func (h ClientLog) forward(ctx context.Context, in requests.ClientLog) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := h.post(ctx, in); err != nil {
		h.Log.WarnContext(ctx, "client-log: forward failed", "err", err)
	}
}

func (h ClientLog) post(ctx context.Context, in requests.ClientLog) error {
	body, err := json.Marshal(in)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.URL+"/log", bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client-Logger-Key", h.Key)

	res, err := h.Client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("client-logger answered %d", res.StatusCode)
	}

	return nil
}
