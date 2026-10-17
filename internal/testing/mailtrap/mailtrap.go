// Package mailtrap reads the Mailtrap Email Sandbox from tests: list
// captured messages, wait for one by recipient and subject, fetch its body
// and clean the sandbox. It uses only net/http (no Mailtrap SDK).
//
// API: https://docs.mailtrap.io/developers/email-sandbox/messages
package mailtrap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

// DefaultBaseURL is the Mailtrap API origin.
const DefaultBaseURL = "https://mailtrap.io"

// ErrTimeout is returned by WaitFor when no matching message arrived.
var ErrTimeout = errors.New("mailtrap: message not received in time")

// Client talks to one sandbox.
type Client struct {
	BaseURL   string
	Token     string
	SandboxID string
	HTTP      *http.Client
	// Poll is the delay between WaitFor attempts (the sandbox API is rate
	// limited per sandbox, so keep it at a second or more).
	Poll time.Duration
}

// FromEnv builds a client from MAILTRAP_API_TOKEN and MAILTRAP_SANDBOX_ID;
// ok is false when either is unset (callers skip their test).
func FromEnv() (c *Client, ok bool) {
	token, sandbox := os.Getenv("MAILTRAP_API_TOKEN"), os.Getenv("MAILTRAP_SANDBOX_ID")
	if token == "" || sandbox == "" {
		return nil, false
	}

	return &Client{BaseURL: DefaultBaseURL, Token: token, SandboxID: sandbox}, true
}

// Message is a captured email (the fields tests need).
type Message struct {
	ID        int64     `json:"id"`
	Subject   string    `json:"subject"`
	ToEmail   string    `json:"to_email"`
	FromEmail string    `json:"from_email"`
	SentAt    time.Time `json:"sent_at"`
}

// Messages lists the latest messages, filtered by Mailtrap's search
// (subject, recipient address or name) when search is not empty.
func (c *Client) Messages(ctx context.Context, search string) ([]Message, error) {
	q := url.Values{}
	if search != "" {
		q.Set("search", search)
	}

	var out []Message
	if err := c.getJSON(ctx, "/messages?"+q.Encode(), &out); err != nil {
		return nil, err
	}

	return out, nil
}

// WaitFor polls until a message to recipient whose subject contains
// subject arrives, or timeout passes.
func (c *Client) WaitFor(ctx context.Context, recipient, subject string, timeout time.Duration) (Message, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	poll := c.Poll
	if poll <= 0 {
		poll = 2 * time.Second
	}

	for {
		msgs, err := c.Messages(ctx, recipient)
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			return Message{}, err
		}

		for _, m := range msgs {
			if strings.EqualFold(m.ToEmail, recipient) && strings.Contains(m.Subject, subject) {
				return m, nil
			}
		}

		select {
		case <-ctx.Done():
			return Message{}, ErrTimeout
		case <-time.After(poll):
		}
	}
}

// HTML returns the HTML body of message id.
func (c *Client) HTML(ctx context.Context, id int64) (string, error) {
	return c.getText(ctx, fmt.Sprintf("/messages/%d/body.html", id))
}

// Text returns the plain-text body of message id.
func (c *Client) Text(ctx context.Context, id int64) (string, error) {
	return c.getText(ctx, fmt.Sprintf("/messages/%d/body.txt", id))
}

// Clean deletes every message in the sandbox (run before a suite).
func (c *Client) Clean(ctx context.Context) error {
	res, err := c.do(ctx, http.MethodPatch, "/clean")
	if err != nil {
		return err
	}

	return res.Body.Close()
}

var hrefRe = regexp.MustCompile(`href="([^"]+)"`)

// FindLink returns the first href in html that contains part, with HTML
// entities in the query string (&amp;) decoded.
func FindLink(html, part string) (string, bool) {
	for _, m := range hrefRe.FindAllStringSubmatch(html, -1) {
		link := strings.ReplaceAll(m[1], "&amp;", "&")
		if strings.Contains(link, part) {
			return link, true
		}
	}

	return "", false
}

func (c *Client) getJSON(ctx context.Context, path string, v any) error {
	res, err := c.do(ctx, http.MethodGet, path)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(v); err != nil {
		return fmt.Errorf("mailtrap: decode %s: %w", path, err)
	}

	return nil
}

func (c *Client) getText(ctx context.Context, path string) (string, error) {
	res, err := c.do(ctx, http.MethodGet, path)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	b, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return "", fmt.Errorf("mailtrap: read %s: %w", path, err)
	}

	return string(b), nil
}

func (c *Client) do(ctx context.Context, method, path string) (*http.Response, error) {
	base := c.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}

	endpoint := strings.TrimSuffix(base, "/") + "/api/sandboxes/" + url.PathEscape(c.SandboxID) + path

	req, err := http.NewRequestWithContext(ctx, method, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("mailtrap: %w", err)
	}

	req.Header.Set("Api-Token", c.Token)
	req.Header.Set("Accept", "application/json")

	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 15 * time.Second}
	}

	res, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mailtrap: %s %s: %w", method, path, err)
	}

	if res.StatusCode >= http.StatusBadRequest {
		_ = res.Body.Close()

		return nil, fmt.Errorf("mailtrap: %s %s: status %d", method, path, res.StatusCode)
	}

	return res, nil
}
