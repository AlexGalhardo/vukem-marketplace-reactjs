package payments

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"github.com/AlexGalhardo/vukem/orm"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/dbx"
)

// Shop is the payments view of a seller's shop.
type Shop struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	Slug            string `json:"slug"`
	StripeAccountID string `json:"-"`
	StripeReady     bool   `json:"stripeReady"`
}

// ShopOf returns the shop owned by userID, or orm.ErrNotFound.
func (s *Service) ShopOf(ctx context.Context, userID int64) (Shop, error) {
	var (
		sh      Shop
		account sql.NullString
	)

	err := dbx.QueryRow(ctx, s.DB, `SELECT id, name, slug, stripe_account_id, stripe_ready FROM shops WHERE owner_id = ?`,
		userID).
		Scan(&sh.ID, &sh.Name, &sh.Slug, &account, &sh.StripeReady)
	if errors.Is(err, sql.ErrNoRows) {
		return Shop{}, orm.ErrNotFound
	}

	if err != nil {
		return Shop{}, fmt.Errorf("shop of user %d: %w", userID, err)
	}

	sh.StripeAccountID = account.String

	return sh, nil
}

// Onboard creates the shop's Express account on first use and returns a
// fresh Account Link to Stripe's onboarding.
func (s *Service) Onboard(ctx context.Context, sh Shop, email string) (string, error) {
	if s.Stripe == nil {
		return "", ErrCardDisabled
	}

	if sh.StripeAccountID == "" {
		acct, err := s.Stripe.CreateExpressAccount(
			ctx,
			ExpressAccountParams{Email: email, Country: "BR", ShopID: sh.ID},
			"account-"+strconv.FormatInt(sh.ID, 10),
		)
		if err != nil {
			return "", fmt.Errorf("create express account for shop %d: %w", sh.ID, err)
		}

		if _, err := dbx.Exec(ctx, s.DB, `UPDATE shops SET stripe_account_id = ?, updated_at = ?
			WHERE id = ? AND stripe_account_id IS NULL`, acct.ID, s.now(), sh.ID); err != nil {
			return "", fmt.Errorf("store express account: %w", err)
		}

		// A concurrent request may have stored its account first; use whatever is stored.
		if sh, err = s.shopByID(ctx, sh.ID); err != nil {
			return "", err
		}
	}

	return s.Stripe.CreateAccountLink(ctx, sh.StripeAccountID,
		s.Config.AppURL+"/seller/payouts?stripe=refresh", s.Config.AppURL+"/seller/payouts?stripe=return")
}

func (s *Service) shopByID(ctx context.Context, id int64) (Shop, error) {
	var (
		sh      Shop
		account sql.NullString
	)

	err := dbx.QueryRow(ctx, s.DB, `SELECT id, name, slug, stripe_account_id, stripe_ready FROM shops WHERE id = ?`, id).
		Scan(&sh.ID, &sh.Name, &sh.Slug, &account, &sh.StripeReady)
	if err != nil {
		return Shop{}, fmt.Errorf("shop %d: %w", id, err)
	}

	sh.StripeAccountID = account.String

	return sh, nil
}

// SyncAccount re-reads a connected account from Stripe (events may arrive
// out of order, so the event body is never trusted for state) and stores
// whether it can receive transfers.
func (s *Service) SyncAccount(ctx context.Context, accountID string) error {
	if s.Stripe == nil {
		return ErrCardDisabled
	}

	acct, err := s.Stripe.GetAccount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("get account %s: %w", accountID, err)
	}

	_, err = dbx.Exec(ctx, s.DB, `UPDATE shops SET stripe_ready = ?, updated_at = ? WHERE stripe_account_id = ?`,
		acct.Ready, s.now(), accountID)
	if err != nil {
		return fmt.Errorf("sync account %s: %w", accountID, err)
	}

	return nil
}
