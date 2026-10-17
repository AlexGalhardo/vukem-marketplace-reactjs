// Package alerts is the Telegram ops channel: contact form messages and
// rate-limited 5xx alerts, both sent from queue jobs. Without
// TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID every send is a no-op.
package alerts

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/AlexGalhardo/vukem/config"
)

// DefaultTelegramURL is the Bot API base URL.
const DefaultTelegramURL = "https://api.telegram.org"

// maxMessage is Telegram's message length limit (characters).
const maxMessage = 4096

// TelegramConfig is read from TELEGRAM_* variables (the same ones vukem
// reads for its log handler).
type TelegramConfig struct {
	BotToken config.Secret `env:"TELEGRAM_BOT_TOKEN"`
	ChatID   string        `env:"TELEGRAM_CHAT_ID"`
	// APIURL is the Bot API base URL; tests point it at a fake.
	APIURL string `env:"TELEGRAM_API_URL" default:"https://api.telegram.org"`
}

// ErrTelegramStatus is wrapped by Send when the Bot API answers non-2xx.
var ErrTelegramStatus = errors.New("telegram: unexpected status")

// Telegram sends messages synchronously, so a queue job can retry a
// failed send.
//
// workaround(vukem): logging/telegram.Handler.Send and Contact are
// fire-and-forget (no error, in-memory queue), so a queued job built on
// them could never retry a lost message.
type Telegram struct {
	token  config.Secret
	chatID string
	base   string
	http   *http.Client
}

// NewTelegram returns a client; it is disabled when the token or the chat
// is missing.
func NewTelegram(cfg TelegramConfig) *Telegram {
	base := strings.TrimSuffix(cfg.APIURL, "/")
	if base == "" {
		base = DefaultTelegramURL
	}

	return &Telegram{
		token: cfg.BotToken, chatID: cfg.ChatID, base: base,
		http: &http.Client{Timeout: 10 * time.Second},
	}
}

// Enabled reports whether messages are sent.
func (t *Telegram) Enabled() bool {
	return t != nil && !t.token.IsZero() && t.chatID != ""
}

// Send posts an HTML message (callers escape user content with
// html.EscapeString). Errors never contain the bot token.
func (t *Telegram) Send(ctx context.Context, htmlText string) error {
	if !t.Enabled() {
		return nil
	}

	body, err := json.Marshal(map[string]any{
		"chat_id":                  t.chatID,
		"text":                     truncate(htmlText, maxMessage),
		"parse_mode":               "HTML",
		"disable_web_page_preview": true,
	})
	if err != nil {
		return fmt.Errorf("telegram: encode: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		t.base+"/bot"+t.token.Reveal()+"/sendMessage", bytes.NewReader(body))
	if err != nil {
		return errors.New("telegram: invalid API URL")
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := t.http.Do(req)
	if err != nil {
		// *url.Error quotes the URL, which holds the token.
		if ue, ok := errors.AsType[*url.Error](err); ok {
			err = ue.Err
		}

		return fmt.Errorf("telegram: send: %w", err)
	}
	defer resp.Body.Close()

	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("%w %d", ErrTelegramStatus, resp.StatusCode)
	}

	return nil
}

// truncate cuts s to at most n runes.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}

	return string(r[:n-1]) + "…"
}
