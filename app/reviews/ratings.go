package reviews

import (
	"context"
	"fmt"
	"strings"

	"github.com/AlexGalhardo/vukem/database"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/dbx"
)

// AvgX100 is the average of ratings summing to sum over count reviews,
// times 100, rounded half up in integers (4.255 → 426): what
// products.rating_avg_x100 stores. Zero reviews average 0.
func AvgX100(sum, count int64) int64 {
	if count <= 0 {
		return 0
	}

	return (sum*200 + count) / (2 * count)
}

// ratingsSQL recomputes the stored rating of the products it is applied
// to from their visible reviews; the arithmetic is AvgX100's, in SQL
// integers (Postgres bigint, SQLite integer).
const ratingsSQL = `UPDATE products SET
	rating_count = (SELECT COUNT(*) FROM reviews r WHERE r.product_id = products.id AND r.hidden_at IS NULL),
	rating_avg_x100 = COALESCE((SELECT (SUM(r.rating) * 200 + COUNT(*)) / (2 * COUNT(*)) FROM reviews r
		WHERE r.product_id = products.id AND r.hidden_at IS NULL HAVING COUNT(*) > 0), 0)`

// recalcBatch bounds the placeholders of one UPDATE.
const recalcBatch = 500

// Recalculate rebuilds rating_avg_x100 and rating_count of productIDs
// (every product when none is given) from their visible reviews, through
// ex: the caller's transaction or the database. Batch importers (the demo
// seeder) call it once after inserting reviews with plain SQL.
func Recalculate(ctx context.Context, ex database.Executor, productIDs ...int64) error {
	if len(productIDs) == 0 {
		if _, err := ex.ExecContext(ctx, ratingsSQL); err != nil {
			return fmt.Errorf("recalculate all ratings: %w", err)
		}

		return nil
	}

	for start := 0; start < len(productIDs); start += recalcBatch {
		batch := productIDs[start:min(start+recalcBatch, len(productIDs))]

		args := make([]any, len(batch))
		for i, id := range batch {
			args[i] = id
		}

		q, err := database.RebindChecked(ex.Dialect(),
			ratingsSQL+` WHERE id IN (`+strings.TrimSuffix(strings.Repeat("?, ", len(batch)), ", ")+`)`)
		if err != nil {
			return err
		}

		if _, err := ex.ExecContext(ctx, q, args...); err != nil {
			return fmt.Errorf("recalculate ratings of %d products: %w", len(batch), err)
		}
	}

	return nil
}

// RecalculateRatings is Recalculate on the service database.
func (s *Service) RecalculateRatings(ctx context.Context, productIDs ...int64) error {
	return Recalculate(ctx, s.DB, productIDs...)
}

// lockProduct takes the product row lock that serializes rating changes:
// two reviews committed at once must both be counted.
func lockProduct(ctx context.Context, tx *database.Tx, productID int64) error {
	var id int64

	err := dbx.QueryRow(ctx, tx, `SELECT id FROM products WHERE id = ?`+dbx.ForUpdate(tx.Dialect()), productID).
		Scan(&id)
	if err != nil {
		return fmt.Errorf("lock product %d: %w", productID, err)
	}

	return nil
}
