package webhooks

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"slices"
	"strings"

	"github.com/AlexGalhardo/vukem/errs"
	"github.com/AlexGalhardo/vukem/httpclient"
	"github.com/AlexGalhardo/vukem/i18n"
	"github.com/AlexGalhardo/vukem/orm"
	"github.com/AlexGalhardo/vukem/webhook"
)

// ErrNoShop is returned by ShopOf for a user without a shop.
var ErrNoShop = errors.New("webhooks: user has no shop")

// Validation messages are i18n keys, translated with the request locale.
const (
	msgURLInvalid  = "webhooks.errors.url_invalid"
	msgURLHTTPS    = "webhooks.errors.url_https"
	msgURLInternal = "webhooks.errors.url_internal"
	msgEvents      = "webhooks.errors.events"
	msgLimit       = "webhooks.errors.limit"
)

// ShopOf returns the id of the shop owned by userID.
func (s *Service) ShopOf(ctx context.Context, userID int64) (int64, error) {
	var id int64

	err := s.db.QueryRowContext(ctx, "SELECT id FROM shops WHERE owner_id = "+s.db.Dialect().Placeholder(1), userID).
		Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNoShop
	}

	if err != nil {
		return 0, fmt.Errorf("webhooks: shop of user %d: %w", userID, err)
	}

	return id, nil
}

// List returns the shop's endpoints, newest first.
func (s *Service) List(ctx context.Context, shopID int64) ([]Endpoint, error) {
	return orm.QueryOf[Endpoint](s.db).Where("shop_id", shopID).OrderBy("id", orm.Desc).Get(ctx)
}

// Find returns one endpoint of the shop; another shop's endpoint is
// orm.ErrNotFound (404), never a 403 that would confirm it exists.
func (s *Service) Find(ctx context.Context, shopID, id int64) (Endpoint, error) {
	return orm.QueryOf[Endpoint](s.db).Where("id", id).Where("shop_id", shopID).First(ctx)
}

// Delete removes one endpoint of the shop (its deliveries cascade, and
// pending retries stop at their next lookup).
func (s *Service) Delete(ctx context.Context, shopID, id int64) error {
	n, err := orm.QueryOf[Endpoint](s.db).Where("id", id).Where("shop_id", shopID).Delete(ctx)
	if err != nil {
		return err
	}

	if n == 0 {
		return orm.ErrNotFound
	}

	return nil
}

// Recent returns the latest deliveries of an endpoint, newest first.
// Callers check ownership with Find first.
func (s *Service) Recent(ctx context.Context, endpointID int64, limit int) ([]Delivery, error) {
	return orm.QueryOf[Delivery](s.db).Where("endpoint_id", endpointID).OrderBy("id", orm.Desc).Limit(limit).Get(ctx)
}

// Create registers an endpoint and returns it with its signing secret,
// which is shown to the seller once and then only stored encrypted.
func (s *Service) Create(ctx context.Context, shopID int64, rawURL string, events []string) (Endpoint, string, error) {
	v := errs.NewValidation()

	if err := s.CheckURL(rawURL); err != nil {
		v = v.Add("url", i18n.T(ctx, err.Error()))
	}

	if len(events) == 0 ||
		slices.ContainsFunc(events, func(e string) bool { return !slices.Contains(AllEvents(), e) }) {
		v = v.Add("events", i18n.T(ctx, msgEvents))
	}

	n, err := orm.QueryOf[Endpoint](s.db).Where("shop_id", shopID).Count(ctx)
	if err != nil {
		return Endpoint{}, "", err
	}

	if n >= MaxEndpoints {
		v = v.Add("url", i18n.T(ctx, msgLimit, "max", MaxEndpoints))
	}

	if !v.Empty() {
		return Endpoint{}, "", v
	}

	secret, err := webhook.NewSecret()
	if err != nil {
		return Endpoint{}, "", fmt.Errorf("webhooks: secret: %w", err)
	}

	// Keep AllEvents order so the stored list is canonical.
	subscribed := slices.DeleteFunc(AllEvents(), func(e string) bool { return !slices.Contains(events, e) })

	ep := Endpoint{
		ShopID: shopID, URL: rawURL, Secret: orm.Encrypted[string]{V: secret},
		Events: strings.Join(subscribed, " "), Active: true,
	}

	if err := orm.Create(ctx, s.db, &ep); err != nil {
		return Endpoint{}, "", fmt.Errorf("webhooks: create endpoint: %w", err)
	}

	return ep, secret, nil
}

// urlError is a validation failure whose message is an i18n key.
type urlError string

func (e urlError) Error() string { return string(e) }

// CheckURL rejects what can never be a public HTTPS endpoint: other
// schemes, credentials in the URL, and literal or well-known internal
// hosts. Hostnames are resolved again at every attempt by
// httpclient.Safe, which also blocks names that resolve (or are
// rebound) to internal addresses and redirects to them.
func (s *Service) CheckURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.Hostname() == "" || u.User != nil || len(raw) > 2048 {
		return urlError(msgURLInvalid)
	}

	switch {
	case u.Scheme == "https":
	case u.Scheme == "http" && s.opts.InsecureAllowHTTP:
	default:
		return urlError(msgURLHTTPS)
	}

	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))

	if ip, err := netip.ParseAddr(host); err == nil {
		if !httpclient.Allowed(ip.Unmap(), s.opts.InsecureAllow) {
			return urlError(msgURLInternal)
		}

		return nil
	}

	if !strings.Contains(host, ".") || host == "localhost" ||
		slices.ContainsFunc([]string{".localhost", ".local", ".internal", ".home.arpa"}, func(suf string) bool {
			return strings.HasSuffix(host, suf)
		}) {
		return urlError(msgURLInternal)
	}

	return nil
}
