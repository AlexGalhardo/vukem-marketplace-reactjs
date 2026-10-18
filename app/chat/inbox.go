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

// ShopRef names a shop.
type ShopRef struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// ProductRef names a product.
type ProductRef struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

// Summary is one conversation in an inbox.
type Summary struct {
	ID int64 `json:"id"`
	// Role is the viewer's side; With is how they see the other side (the
	// shop's name for buyers, the buyer's first name for sellers).
	Role          string      `json:"role"`
	With          string      `json:"with"`
	Shop          ShopRef     `json:"shop"`
	Product       *ProductRef `json:"product"`
	LastMessage   string      `json:"last_message"`
	LastFromMe    bool        `json:"last_from_me"`
	LastMessageAt *time.Time  `json:"last_message_at"`
	Unread        int64       `json:"unread"`
}

// Page is one page of results.
type Page[T any] = catalog.Page[T]

const summarySelect = `SELECT c.id, c.buyer_id, s.owner_id, s.slug, s.name, u.name,
	p.slug, p.title, c.last_message_at, m.body, m.sender_id,
	(SELECT COUNT(*) FROM chat_messages x WHERE x.conversation_id = c.id AND x.sender_id <> ? AND x.id > %s)
	FROM conversations c
	JOIN shops s ON s.id = c.shop_id
	JOIN users u ON u.id = c.buyer_id
	LEFT JOIN products p ON p.id = c.product_id
	JOIN chat_messages m ON m.id = c.last_message_id`

// Inbox lists userID's conversations on role's side that have messages,
// newest first. Sellers without a shop get ErrNoShop.
func (s *Service) Inbox(ctx context.Context, userID int64, role string, page int) (Page[Summary], error) {
	page = max(page, 1)

	var (
		where string
		arg   any
	)

	if role == RoleSeller {
		shop, err := orm.QueryOf[catalog.Shop](s.db).Select("id").Where("owner_id", userID).First(ctx)
		if errors.Is(err, orm.ErrNotFound) {
			return Page[Summary]{}, ErrNoShop
		}

		if err != nil {
			return Page[Summary]{}, fmt.Errorf("load shop of user %d: %w", userID, err)
		}

		where, arg = "c.shop_id = ?", shop.ID
	} else {
		role = RoleBuyer
		where, arg = "c.buyer_id = ?", userID
	}

	var total int64
	if err := dbx.QueryRow(ctx, s.db, `SELECT COUNT(*) FROM conversations c WHERE `+where+
		` AND c.last_message_id > 0`, arg).Scan(&total); err != nil {
		return Page[Summary]{}, fmt.Errorf("count conversations: %w", err)
	}

	q := fmt.Sprintf(summarySelect, "c."+readColumn(role)) + ` WHERE ` + where +
		` ORDER BY c.last_message_at DESC, c.id DESC LIMIT ? OFFSET ?`

	items, err := s.summaries(ctx, q, userID, role, userID, arg, PerPage, (page-1)*PerPage)
	if err != nil {
		return Page[Summary]{}, err
	}

	return pageOf(items, total, page), nil
}

func pageOf[T any](items []T, total int64, page int) Page[T] {
	return Page[T]{
		Items: items, Total: total, Page: page, PerPage: PerPage,
		LastPage: max(1, int((total+PerPage-1)/PerPage)),
	}
}

func (s *Service) summaries(ctx context.Context, q string, viewer int64, role string, args ...any) ([]Summary, error) {
	q, err := database.RebindChecked(s.db.Dialect(), q)
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	defer rows.Close()

	out := []Summary{}

	for rows.Next() {
		var (
			sum           Summary
			buyerID       int64
			ownerID       int64
			buyerName     string
			pSlug, pTitle sql.Null[string]
			at            orm.NullTime
			senderID      int64
		)

		if err := rows.Scan(&sum.ID, &buyerID, &ownerID, &sum.Shop.Slug, &sum.Shop.Name, &buyerName,
			&pSlug, &pTitle, &at, &sum.LastMessage, &senderID, &sum.Unread); err != nil {
			return nil, fmt.Errorf("scan conversation: %w", err)
		}

		// The admin view (role "") names the buyer, like the seller's.
		sum.Role = role

		if role == RoleBuyer {
			sum.With = sum.Shop.Name
		} else {
			sum.With = FirstName(buyerName)
		}

		if pSlug.Valid {
			sum.Product = &ProductRef{Slug: pSlug.V, Title: pTitle.V}
		}

		if at.Valid {
			sum.LastMessageAt = &at.V
		}

		sum.LastFromMe = senderID == viewer

		out = append(out, sum)
	}

	return out, rows.Err()
}

// Detail is a conversation as its page shows it.
type Detail struct {
	ID      int64       `json:"id"`
	Role    string      `json:"role"`
	With    string      `json:"with"`
	WithID  int64       `json:"with_id"`
	Shop    ShopRef     `json:"shop"`
	Product *ProductRef `json:"product"`
	// MyLastReadID and TheirLastReadID are the read receipts.
	MyLastReadID    int64 `json:"my_last_read_id"`
	TheirLastReadID int64 `json:"their_last_read_id"`
	// Buyer is the buyer's first name (admin view).
	Buyer string `json:"buyer"`
}

// Describe returns the page view of thread t for role (RoleBuyer for the
// admin's read-only view).
func (s *Service) Describe(ctx context.Context, t Thread, role string) (Detail, error) {
	var (
		buyerName     string
		pSlug, pTitle sql.Null[string]
		d             = Detail{ID: t.ID, Role: role}
	)

	err := dbx.QueryRow(ctx, s.db, `SELECT s.slug, s.name, u.name, p.slug, p.title
		FROM conversations c JOIN shops s ON s.id = c.shop_id JOIN users u ON u.id = c.buyer_id
		LEFT JOIN products p ON p.id = c.product_id WHERE c.id = ?`, t.ID).
		Scan(&d.Shop.Slug, &d.Shop.Name, &buyerName, &pSlug, &pTitle)
	if err != nil {
		return Detail{}, fmt.Errorf("describe conversation %d: %w", t.ID, err)
	}

	d.Buyer = FirstName(buyerName)

	if pSlug.Valid {
		d.Product = &ProductRef{Slug: pSlug.V, Title: pTitle.V}
	}

	if role == RoleSeller {
		d.With, d.WithID = d.Buyer, t.BuyerID
		d.MyLastReadID, d.TheirLastReadID = t.SellerLastReadID, t.BuyerLastReadID
	} else {
		d.With, d.WithID = d.Shop.Name, t.OwnerID
		d.MyLastReadID, d.TheirLastReadID = t.BuyerLastReadID, t.SellerLastReadID
	}

	return d, nil
}

// AdminInbox lists every conversation with messages, newest first, for
// moderation (read-only).
func (s *Service) AdminInbox(ctx context.Context, page int) (Page[Summary], error) {
	page = max(page, 1)

	var total int64
	if err := dbx.QueryRow(ctx, s.db, `SELECT COUNT(*) FROM conversations WHERE last_message_id > 0`).
		Scan(&total); err != nil {
		return Page[Summary]{}, fmt.Errorf("count conversations: %w", err)
	}

	// Unread is meaningless for the admin: count against a viewer id of 0
	// and a read receipt past every message.
	q := fmt.Sprintf(summarySelect, "c.last_message_id") +
		` ORDER BY c.last_message_at DESC, c.id DESC LIMIT ? OFFSET ?`

	items, err := s.summaries(ctx, q, 0, "", 0, PerPage, (page-1)*PerPage)
	if err != nil {
		return Page[Summary]{}, err
	}

	return pageOf(items, total, page), nil
}

// AdminThread loads any conversation (admin moderation, read-only).
func (s *Service) AdminThread(ctx context.Context, id int64) (Thread, error) {
	return s.thread(ctx, s.db, id, false)
}
