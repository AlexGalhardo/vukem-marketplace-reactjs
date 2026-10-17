package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/AlexGalhardo/vukem/orm"
)

// ShopView is a shop as pages and APIs show it.
type ShopView struct {
	ID          int64     `json:"id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	LogoURL     string    `json:"logo_url"`
	ShipFromUF  string    `json:"ship_from_uf"`
	Status      string    `json:"status"`
	StripeReady bool      `json:"stripe_ready"`
	Since       time.Time `json:"since"`
}

func shopView(s Shop) ShopView {
	v := ShopView{
		ID: s.ID, Slug: s.Slug, Name: s.Name, Description: s.Description.V,
		ShipFromUF: s.ShipFromUF, Status: s.Status, StripeReady: s.StripeReady, Since: s.CreatedAt,
	}
	if s.LogoKey.Valid {
		v.LogoURL = "/media/" + s.LogoKey.V // the original until its thumbnail replaces it
	}

	return v
}

func nullString(s string) sql.Null[string] {
	return sql.Null[string]{V: s, Valid: s != ""}
}

// OpenShop creates the user's shop. Verified-email gating happens in the
// route (auth.Verified); one shop per user is enforced here and by the
// unique owner_id index.
func (s *Service) OpenShop(ctx context.Context, userID int64, in ShopInput) (ShopView, error) {
	if _, err := s.ownShop(ctx, userID); err == nil {
		return ShopView{}, ErrShopExists
	} else if !errors.Is(err, ErrNoShop) {
		return ShopView{}, err
	}

	shop := Shop{
		OwnerID: userID, Name: strings.TrimSpace(in.Name), Description: nullString(in.Description),
		Status: ShopActive, ShipFromUF: in.ShipFromUF,
	}

	err := insertWithSlug(ctx, in.Name, func(ctx context.Context, slug string) error {
		shop.Slug = slug

		err := orm.Create(ctx, s.db, &shop)
		if errors.Is(err, orm.ErrConflict) {
			// The conflict may be the owner index (a concurrent request).
			if _, ownErr := s.ownShop(ctx, userID); ownErr == nil {
				return ErrShopExists
			}
		}

		return err
	})
	if err != nil {
		return ShopView{}, fmt.Errorf("open shop: %w", err)
	}

	return shopView(shop), nil
}

// ownShop loads the shop owned by userID (ErrNoShop when there is none).
func (s *Service) ownShop(ctx context.Context, userID int64) (Shop, error) {
	shop, err := orm.QueryOf[Shop](s.db).Where("owner_id", userID).First(ctx)
	if errors.Is(err, orm.ErrNotFound) {
		return Shop{}, ErrNoShop
	}

	if err != nil {
		return Shop{}, fmt.Errorf("load shop of user %d: %w", userID, err)
	}

	return shop, nil
}

// MyShop returns the acting user's shop, or ErrNoShop.
func (s *Service) MyShop(ctx context.Context, userID int64) (ShopView, error) {
	shop, err := s.ownShop(ctx, userID)
	if err != nil {
		return ShopView{}, err
	}

	return shopView(shop), nil
}

// UpdateShop changes the branding of the acting user's shop. The slug is
// stable (links keep working after a rename).
func (s *Service) UpdateShop(ctx context.Context, userID int64, in ShopInput) (ShopView, error) {
	shop, err := s.ownShop(ctx, userID)
	if err != nil {
		return ShopView{}, err
	}

	shop.Name, shop.Description, shop.ShipFromUF = strings.TrimSpace(in.Name), nullString(in.Description), in.ShipFromUF

	if _, err := orm.QueryOf[Shop](s.db).Where("id", shop.ID).Update(ctx, orm.Set{
		"name": shop.Name, "description": shop.Description, "ship_from_uf": shop.ShipFromUF,
	}); err != nil {
		return ShopView{}, fmt.Errorf("update shop %d: %w", shop.ID, err)
	}

	return shopView(shop), nil
}

// ShopBySlug returns an active shop; suspended or unknown shops are
// ErrNotFound.
func (s *Service) ShopBySlug(ctx context.Context, slug string) (ShopView, error) {
	shop, err := orm.QueryOf[Shop](s.db).Where("slug", slug).Where("status", ShopActive).First(ctx)
	if errors.Is(err, orm.ErrNotFound) {
		return ShopView{}, ErrNotFound
	}

	if err != nil {
		return ShopView{}, fmt.Errorf("load shop %q: %w", slug, err)
	}

	return shopView(shop), nil
}
