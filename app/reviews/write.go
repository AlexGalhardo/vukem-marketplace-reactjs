package reviews

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/AlexGalhardo/vukem/database"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/dbx"
)

// eligibleFrom selects the order lines the user bought, with their product, in
// a seller order that can be reviewed, from a shop the user does not own.
// It is the only eligibility rule; Eligible, Create and PendingItem use it.
const eligibleFrom = ` FROM order_items oi
	JOIN seller_orders so ON so.id = oi.seller_order_id
	JOIN orders o ON o.id = so.order_id
	JOIN products p ON p.id = oi.product_id
	JOIN shops s ON s.id = p.shop_id
	LEFT JOIN reviews r ON r.order_item_id = oi.id
	WHERE o.buyer_id = ? AND s.owner_id <> ? AND so.status IN (?, ?, ?)`

// Item is an order line the user may review.
type Item struct {
	OrderItemID  int64  `json:"order_item_id"`
	ProductID    int64  `json:"product_id"`
	ProductSlug  string `json:"product_slug"`
	ProductTitle string `json:"product_title"`
	// ReviewID is the line's review, 0 while there is none.
	ReviewID int64 `json:"review_id"`
}

func eligible(ctx context.Context, ex database.Executor, userID, orderItemID int64) (Item, error) {
	args := append([]any{userID, userID}, statusArgs()...)

	var it Item

	err := dbx.QueryRow(ctx, ex, `SELECT oi.id, p.id, p.slug, p.title, COALESCE(r.id, 0)`+eligibleFrom+` AND oi.id = ?`, append(args, orderItemID)...).
		Scan(&it.OrderItemID, &it.ProductID, &it.ProductSlug, &it.ProductTitle, &it.ReviewID)
	if errors.Is(err, sql.ErrNoRows) {
		return Item{}, ErrNotFound
	}

	if err != nil {
		return Item{}, fmt.Errorf("load order item %d: %w", orderItemID, err)
	}

	return it, nil
}

// Eligible returns order line orderItemID when userID may review it (it
// may already have a review: see Item.ReviewID); ErrNotFound otherwise.
func (s *Service) Eligible(ctx context.Context, userID, orderItemID int64) (Item, error) {
	return eligible(ctx, s.DB, userID, orderItemID)
}

// PendingItem returns userID's oldest order line of productID that is
// eligible and not reviewed yet, or 0 when there is none.
func (s *Service) PendingItem(ctx context.Context, userID, productID int64) (int64, error) {
	if userID == 0 {
		return 0, nil
	}

	args := append([]any{userID, userID}, statusArgs()...)

	var id int64

	err := dbx.QueryRow(ctx, s.DB, `SELECT oi.id`+eligibleFrom+` AND p.id = ? AND r.id IS NULL ORDER BY oi.id LIMIT 1`, append(args, productID)...).
		Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}

	if err != nil {
		return 0, fmt.Errorf("pending review item of product %d: %w", productID, err)
	}

	return id, nil
}

// Input is a validated review (InputSchema).
type Input struct {
	Rating int    `json:"rating" form:"rating"`
	Title  string `json:"title"  form:"title"`
	Body   string `json:"body"   form:"body"`
	// Recommend is "yes", "no" or "" (not answered).
	Recommend string `json:"recommend" form:"recommend"`
}

func nullString(s string) sql.Null[string] { return sql.Null[string]{V: s, Valid: s != ""} }

func (in Input) recommend() sql.Null[bool] {
	switch in.Recommend {
	case "yes":
		return sql.Null[bool]{V: true, Valid: true}
	case "no":
		return sql.Null[bool]{V: false, Valid: true}
	default:
		return sql.Null[bool]{}
	}
}

// Create stores userID's review of order line orderItemID and updates the
// product's rating in the same transaction. Ineligible lines are
// ErrNotFound; a second review of a line is ErrAlreadyReviewed.
func (s *Service) Create(ctx context.Context, userID, orderItemID int64, in Input) (Item, error) {
	var it Item

	err := s.DB.Tx(ctx, func(tx *database.Tx) error {
		var err error

		if it, err = eligible(ctx, tx, userID, orderItemID); err != nil {
			return err
		}

		if it.ReviewID != 0 {
			return ErrAlreadyReviewed
		}

		if err := lockProduct(ctx, tx, it.ProductID); err != nil {
			return err
		}

		now := s.now()

		err = dbx.QueryRow(ctx, tx, `INSERT INTO reviews (product_id, buyer_id, order_item_id, rating, title, body,
			recommend, helpful_count, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?, ?) RETURNING id`,
			it.ProductID, userID, orderItemID, in.Rating, nullString(in.Title), nullString(in.Body), in.recommend(),
			now, now).Scan(&it.ReviewID)
		if dbx.IsUniqueViolation(err) {
			return ErrAlreadyReviewed
		}

		if err != nil {
			return fmt.Errorf("insert review: %w", err)
		}

		return Recalculate(ctx, tx, it.ProductID)
	})

	return it, err
}

// visibleReviewSQL selects a review that is not hidden, of a product the
// storefront shows, with its author and the shop owner.
const visibleReviewSQL = `SELECT r.buyer_id, s.owner_id FROM reviews r
	JOIN products p ON p.id = r.product_id
	JOIN shops s ON s.id = p.shop_id
	WHERE r.id = ? AND r.hidden_at IS NULL AND p.status = 'published' AND s.status = 'active'`

// Vote records that userID found review reviewID helpful. Voting twice is
// a no-op (it reports false); authors cannot vote their own review.
func (s *Service) Vote(ctx context.Context, userID, reviewID int64) (bool, error) {
	var added bool

	err := s.DB.Tx(ctx, func(tx *database.Tx) error {
		var author, owner int64

		err := dbx.QueryRow(ctx, tx, visibleReviewSQL, reviewID).Scan(&author, &owner)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}

		if err != nil {
			return fmt.Errorf("load review %d: %w", reviewID, err)
		}

		if author == userID {
			return ErrOwnReview
		}

		n, err := dbx.Exec(ctx, tx, `INSERT INTO review_votes (review_id, user_id, created_at) VALUES (?, ?, ?)
			ON CONFLICT (review_id, user_id) DO NOTHING`, reviewID, userID, s.now())
		if err != nil {
			return fmt.Errorf("insert vote: %w", err)
		}

		if n == 0 {
			return nil
		}

		added = true

		_, err = dbx.Exec(ctx, tx, `UPDATE reviews SET helpful_count = helpful_count + 1 WHERE id = ?`, reviewID)

		return err
	})

	return added, err
}

// Reply stores the shop's answer to review reviewID. Only the owner of
// the reviewed product's shop may reply (ErrNotFound for anyone else),
// once (ErrAlreadyReplied).
func (s *Service) Reply(ctx context.Context, sellerID, reviewID int64, body string) error {
	now := s.now()

	// One conditional UPDATE: two racing replies cannot both land.
	n, err := dbx.Exec(ctx, s.DB, `UPDATE reviews SET seller_reply = ?, seller_replied_at = ?, updated_at = ?
		WHERE id = ? AND seller_reply IS NULL AND hidden_at IS NULL
		AND product_id IN (SELECT p.id FROM products p JOIN shops s ON s.id = p.shop_id WHERE s.owner_id = ?)`,
		body, now, now, reviewID, sellerID)
	if err != nil {
		return fmt.Errorf("reply to review %d: %w", reviewID, err)
	}

	if n == 1 {
		return nil
	}

	var replied bool

	err = dbx.QueryRow(ctx, s.DB, `SELECT r.seller_reply IS NOT NULL FROM reviews r
		JOIN products p ON p.id = r.product_id JOIN shops s ON s.id = p.shop_id
		WHERE r.id = ? AND r.hidden_at IS NULL AND s.owner_id = ?`, reviewID, sellerID).Scan(&replied)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}

	if err != nil {
		return fmt.Errorf("load review %d: %w", reviewID, err)
	}

	if replied {
		return ErrAlreadyReplied
	}

	return ErrNotFound
}

// SetHidden hides (with a reason) or shows review reviewID again and
// updates its product's rating in the same transaction. It reports
// whether anything changed; unknown reviews are ErrNotFound.
func (s *Service) SetHidden(ctx context.Context, adminID, reviewID int64, hidden bool, reason string) (bool, error) {
	var changed bool

	err := s.DB.Tx(ctx, func(tx *database.Tx) error {
		var productID int64

		err := dbx.QueryRow(ctx, tx, `SELECT product_id FROM reviews WHERE id = ?`, reviewID).Scan(&productID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}

		if err != nil {
			return fmt.Errorf("load review %d: %w", reviewID, err)
		}

		if err := lockProduct(ctx, tx, productID); err != nil {
			return err
		}

		now := s.now()

		var n int64
		if hidden {
			n, err = dbx.Exec(
				ctx,
				tx,
				`UPDATE reviews SET hidden_at = ?, hidden_by = ?, hidden_reason = ?, updated_at = ?
				WHERE id = ? AND hidden_at IS NULL`,
				now,
				adminID,
				reason,
				now,
				reviewID,
			)
		} else {
			n, err = dbx.Exec(ctx, tx, `UPDATE reviews SET hidden_at = NULL, hidden_by = NULL, hidden_reason = NULL,
				updated_at = ? WHERE id = ? AND hidden_at IS NOT NULL`, now, reviewID)
		}

		if err != nil {
			return fmt.Errorf("moderate review %d: %w", reviewID, err)
		}

		if n == 0 {
			return nil
		}

		changed = true

		return Recalculate(ctx, tx, productID)
	})

	return changed, err
}
