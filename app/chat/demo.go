package chat

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/orm"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/catalog"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/dbx"
)

// DemoLine is one message of a seeded conversation.
type DemoLine struct {
	FromBuyer bool
	Body      string
	At        time.Time
}

// InsertDemo writes a seeded conversation between buyerID and the owner of
// productID's shop, without broadcasts or emails. It checks what the HTTP
// flow guarantees: the buyer is not the shop owner and every line passes
// SendSchema (no contact data). It is idempotent: a conversation that
// already has messages is left alone (false). The seller has read
// everything but the last buyer line, so the demo inbox shows an unread
// badge. Run it in the seeder's transaction (ex).
func InsertDemo(ctx context.Context, ex database.Executor, buyerID, productID int64, lines []DemoLine) (bool, error) {
	if len(lines) == 0 {
		return false, nil
	}

	for i, l := range lines {
		if _, err := SendSchema.Parse(map[string]any{"body": l.Body}); err != nil {
			return false, fmt.Errorf("chat demo line %d: %w", i, err)
		}
	}

	p, err := orm.QueryOf[catalog.Product](ex).Select("id", "shop_id").Find(ctx, productID)
	if err != nil {
		return false, fmt.Errorf("chat demo: load product %d: %w", productID, err)
	}

	shop, err := orm.QueryOf[catalog.Shop](ex).Select("id", "owner_id").Find(ctx, p.ShopID)
	if err != nil {
		return false, fmt.Errorf("chat demo: load shop %d: %w", p.ShopID, err)
	}

	if shop.OwnerID == buyerID {
		return false, errors.New("chat demo: the buyer owns the shop")
	}

	c, err := orm.QueryOf[Conversation](ex).Where("buyer_id", buyerID).Where("shop_id", shop.ID).
		Where("product_id", productID).First(ctx)
	if errors.Is(err, orm.ErrNotFound) {
		c = Conversation{
			CreatedAt: lines[0].At,
			BuyerID:   buyerID, ShopID: shop.ID, ProductID: sql.Null[int64]{V: productID, Valid: true},
		}
		err = orm.Create(ctx, ex, &c)
	}

	if err != nil {
		return false, fmt.Errorf("chat demo: conversation: %w", err)
	}

	if c.LastMessageID > 0 {
		return false, nil
	}

	var (
		last, lastSeller, buyerRead int64
		at                          time.Time
	)

	for _, l := range lines {
		sender := shop.OwnerID
		if l.FromBuyer {
			sender = buyerID
		}

		m := Message{ConversationID: c.ID, SenderID: sender, Body: l.Body}
		m.CreatedAt, m.UpdatedAt = l.At, l.At

		if err := orm.Create(ctx, ex, &m); err != nil {
			return false, fmt.Errorf("chat demo: message: %w", err)
		}

		last, at = m.ID, l.At

		if l.FromBuyer {
			buyerRead = m.ID
		} else {
			lastSeller = m.ID
			buyerRead = m.ID
		}
	}

	sellerRead := lastSeller
	if !lines[len(lines)-1].FromBuyer {
		sellerRead = last
	}

	_, err = dbx.Exec(ctx, ex, `UPDATE conversations SET last_message_id = ?, last_message_at = ?,
		buyer_last_read_id = ?, seller_last_read_id = ?, updated_at = ? WHERE id = ?`,
		last, at, buyerRead, sellerRead, at, c.ID)
	if err != nil {
		return false, fmt.Errorf("chat demo: update conversation: %w", err)
	}

	return true, nil
}
