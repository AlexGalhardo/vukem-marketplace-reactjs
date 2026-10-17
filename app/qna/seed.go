package qna

import (
	"context"
	"errors"
	"fmt"

	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/orm"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/catalog"
)

// InsertDemo batch-inserts questions written by a seeder, without
// notifications. It checks what the HTTP flow guarantees:
//
//   - ShopID is the product's shop (filled in when zero);
//   - the asker is not the shop's owner;
//   - Body passes AskSchema and Answer, when set, AnswerSchema (no contact
//     data);
//   - Answer and AnsweredAt are set together, as are HiddenAt and
//     HiddenReason.
//
// CreatedAt may be set to spread the demo rows over time. Run it in the
// seeder's transaction (ex) on SQLite, which has a single writer.
func InsertDemo(ctx context.Context, ex database.Executor, rows []Question) error {
	if len(rows) == 0 {
		return nil
	}

	ids := make([]any, 0, len(rows))
	for i := range rows {
		ids = append(ids, rows[i].ProductID)
	}

	products, err := orm.QueryOf[catalog.Product](ex).Select("id", "shop_id").WhereIn("id", ids...).Get(ctx)
	if err != nil {
		return fmt.Errorf("qna demo: load products: %w", err)
	}

	shopOf := map[int64]int64{}
	shopIDs := make([]any, 0, len(products))

	for i := range products {
		shopOf[products[i].ID] = products[i].ShopID
		shopIDs = append(shopIDs, products[i].ShopID)
	}

	shops, err := orm.QueryOf[catalog.Shop](ex).Select("id", "owner_id").WhereIn("id", shopIDs...).Get(ctx)
	if err != nil {
		return fmt.Errorf("qna demo: load shops: %w", err)
	}

	ownerOf := map[int64]int64{}
	for i := range shops {
		ownerOf[shops[i].ID] = shops[i].OwnerID
	}

	for i := range rows {
		if err := checkDemo(&rows[i], shopOf, ownerOf); err != nil {
			return fmt.Errorf("qna demo row %d: %w", i, err)
		}
	}

	if err := orm.CreateMany(ctx, ex, rows); err != nil {
		return fmt.Errorf("qna demo: insert: %w", err)
	}

	return nil
}

func checkDemo(r *Question, shopOf, ownerOf map[int64]int64) error {
	shopID, ok := shopOf[r.ProductID]
	if !ok {
		return fmt.Errorf("unknown product %d", r.ProductID)
	}

	if r.ShopID == 0 {
		r.ShopID = shopID
	}

	switch {
	case r.ShopID != shopID:
		return fmt.Errorf("shop %d is not the shop of product %d", r.ShopID, r.ProductID)
	case ownerOf[shopID] == r.AskerID:
		return fmt.Errorf("asker %d owns the shop", r.AskerID)
	case r.Answer.Valid != r.AnsweredAt.Valid:
		return errors.New("answer and answered_at must be set together")
	case r.HiddenAt.Valid != r.HiddenReason.Valid:
		return errors.New("hidden_at and hidden_reason must be set together")
	}

	body, err := AskSchema.Parse(map[string]any{"body": r.Body})
	if err != nil {
		return fmt.Errorf("body: %w", err)
	}

	r.Body = body.Body

	if r.Answer.Valid {
		a, err := AnswerSchema.Parse(map[string]any{"answer": r.Answer.V})
		if err != nil {
			return fmt.Errorf("answer: %w", err)
		}

		r.Answer.V = a.Answer
	}

	return nil
}
