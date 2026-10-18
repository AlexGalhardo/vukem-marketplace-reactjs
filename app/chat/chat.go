// Package chat is the buyer ↔ seller messaging service. A buyer opens a
// conversation with a shop from a product page (or an order); the shop's
// owner answers from the seller inbox. Rules enforced here and nowhere
// else:
//
//   - one conversation per buyer, shop and optional product;
//   - only the buyer and the shop owner are participants; anyone else gets
//     ErrNotFound (never 403, so ids do not leak), and the broadcast
//     channel rules call Participant too;
//   - a shop owner cannot open a conversation with their own shop;
//   - messages are plain text without contact data (SendSchema);
//   - read receipts only move forward and never past the newest message.
//
// Messages live in Postgres next to the users, shops and products they
// reference, so participation, unread counts and the inbox are joins in
// one transaction; MongoDB would add a second store without a query it
// serves better.
package chat

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/AlexGalhardo/vukem/cache"
	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/errs"
	"github.com/AlexGalhardo/vukem/orm"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/catalog"
	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/dbx"
)

var (
	// ErrNotFound: the conversation, product or shop does not exist, is
	// not visible or the user is not a participant (404).
	ErrNotFound = catalog.ErrNotFound
	// ErrOwnShop: a shop owner tried to chat with their own shop (403).
	// The message is an i18n key.
	ErrOwnShop = errs.New(http.StatusForbidden, MsgOwnShop)
	// ErrNoShop: the acting user has no shop (seller inbox).
	ErrNoShop = catalog.ErrNoShop
)

// Sides of a conversation.
const (
	RoleBuyer  = "buyer"
	RoleSeller = "seller"
)

// Broadcast events and channel names (resources/js/lib/chat.ts mirrors
// them).
const (
	EventMessage = "message.created"
	EventRead    = "message.read"
	EventTyping  = "typing"
	EventInbox   = "inbox.updated"
)

// Page sizes.
const (
	PerPage = 20
	// History is how many recent messages a conversation page loads.
	//
	// shortcut: older messages are not reachable from the page; add a
	// "load older" cursor (before=<id>) when conversations get long.
	History = 100
	// SinceLimit caps one catch-up response; the client asks again while
	// it gets a full page.
	SinceLimit = 100
)

// MessagesChannel is the private channel of a conversation's messages and
// PresenceChannel its presence channel.
func MessagesChannel(id int64) string { return fmt.Sprintf("private-chat.%d", id) }

// PresenceChannel is the presence channel ("online", typing peers) of a
// conversation.
func PresenceChannel(id int64) string { return fmt.Sprintf("presence-chat.%d", id) }

// InboxChannel is the private channel of a user's inbox updates.
func InboxChannel(userID int64) string { return fmt.Sprintf("private-inbox.%d", userID) }

// Conversation is one row of conversations.
type Conversation struct {
	orm.Model

	BuyerID          int64           `db:"buyer_id"`
	ShopID           int64           `db:"shop_id"`
	ProductID        sql.Null[int64] `db:"product_id"`
	LastMessageID    int64           `db:"last_message_id"`
	LastMessageAt    orm.NullTime    `db:"last_message_at"`
	BuyerLastReadID  int64           `db:"buyer_last_read_id"`
	SellerLastReadID int64           `db:"seller_last_read_id"`
}

// Table implements orm.Tabler.
func (Conversation) Table() string { return "conversations" }

// Message is one row of chat_messages.
type Message struct {
	orm.Model

	ConversationID int64  `db:"conversation_id"`
	SenderID       int64  `db:"sender_id"`
	Body           string `db:"body"`
}

// Table implements orm.Tabler.
func (Message) Table() string { return "chat_messages" }

// Publisher sends broadcast events (broadcast.Server).
type Publisher interface {
	Publish(ctx context.Context, channel, event string, data any) error
}

// Config holds the service dependencies.
type Config struct {
	DB *database.DB
	// Cache keeps last-seen times and notification claims; it must be
	// shared by every instance (Redis) in production.
	Cache *cache.Cache
	// Publish is nil until the broadcast server exists (SetPublisher);
	// sends then only reach pages through catch-up.
	Publish Publisher
	// Notify dispatches the offline-notification job after NotifyAfter;
	// nil disables emails.
	Notify      func(ctx context.Context, job NotifyJob, delay time.Duration) error
	NotifyAfter time.Duration
	Log         *slog.Logger
	Now         func() time.Time
}

// Service is the chat API used by the controllers, the broadcast rules,
// the notification job and the seeder.
type Service struct {
	db          *database.DB
	cache       *cache.Cache
	publish     Publisher
	notify      func(ctx context.Context, job NotifyJob, delay time.Duration) error
	notifyAfter time.Duration
	log         *slog.Logger
	now         func() time.Time
}

// DefaultNotifyAfter is how long a recipient must be away before an
// unread message is emailed.
const DefaultNotifyAfter = 10 * time.Minute

// New returns the service.
func New(cfg Config) (*Service, error) {
	if cfg.DB == nil || cfg.Cache == nil {
		return nil, errors.New("chat: Config.DB and Config.Cache are required")
	}

	s := &Service{
		db: cfg.DB, cache: cfg.Cache, publish: cfg.Publish, notify: cfg.Notify,
		notifyAfter: cfg.NotifyAfter, log: cfg.Log, now: cfg.Now,
	}

	if s.notifyAfter <= 0 {
		s.notifyAfter = DefaultNotifyAfter
	}

	if s.log == nil {
		s.log = slog.New(slog.DiscardHandler)
	}

	if s.now == nil {
		s.now = time.Now
	}

	return s, nil
}

// SetPublisher attaches the broadcast server, which needs the service for
// its channel rules and so is built after it.
func (s *Service) SetPublisher(p Publisher) { s.publish = p }

// Thread is a conversation with its shop owner: who may read it.
type Thread struct {
	Conversation

	OwnerID int64
}

// Role returns userID's side, or "" when userID is not a participant.
func (t Thread) Role(userID int64) string {
	switch userID {
	case t.BuyerID:
		return RoleBuyer
	case t.OwnerID:
		return RoleSeller
	default:
		return ""
	}
}

// Counterpart is the other participant's user id.
func (t Thread) Counterpart(userID int64) int64 {
	if userID == t.BuyerID {
		return t.OwnerID
	}

	return t.BuyerID
}

func (t Thread) lastReadOf(role string) int64 {
	if role == RoleBuyer {
		return t.BuyerLastReadID
	}

	return t.SellerLastReadID
}

// thread loads a conversation and its shop owner.
func (s *Service) thread(ctx context.Context, ex database.Executor, id int64, lock bool) (Thread, error) {
	q := `SELECT c.id, c.buyer_id, c.shop_id, c.product_id, c.last_message_id, c.last_message_at,
		c.buyer_last_read_id, c.seller_last_read_id, c.created_at, c.updated_at, s.owner_id
		FROM conversations c JOIN shops s ON s.id = c.shop_id WHERE c.id = ?`
	if lock {
		q += dbx.ForUpdate(ex.Dialect())
	}

	var t Thread

	err := dbx.QueryRow(ctx, ex, q, id).Scan(&t.ID, &t.BuyerID, &t.ShopID, &t.ProductID, &t.LastMessageID,
		&t.LastMessageAt, &t.BuyerLastReadID, &t.SellerLastReadID, &t.CreatedAt, &t.UpdatedAt, &t.OwnerID)
	if errors.Is(err, sql.ErrNoRows) {
		return Thread{}, ErrNotFound
	}

	if err != nil {
		return Thread{}, fmt.Errorf("load conversation %d: %w", id, err)
	}

	return t, nil
}

// Participant loads conversation id for userID and returns their role;
// non-participants get ErrNotFound.
func (s *Service) Participant(ctx context.Context, userID, id int64) (Thread, string, error) {
	t, err := s.thread(ctx, s.db, id, false)
	if err != nil {
		return Thread{}, "", err
	}

	role := t.Role(userID)
	if role == "" {
		return Thread{}, "", ErrNotFound
	}

	return t, role, nil
}

// StartWithProduct returns buyerID's conversation about the visible
// product at slug, creating it on first use.
func (s *Service) StartWithProduct(ctx context.Context, buyerID int64, slug string) (Conversation, error) {
	p, err := orm.QueryOf[catalog.Product](s.db).Where("slug", slug).Where("status", catalog.StatusPublished).First(ctx)
	if errors.Is(err, orm.ErrNotFound) {
		return Conversation{}, ErrNotFound
	}

	if err != nil {
		return Conversation{}, fmt.Errorf("load product %q: %w", slug, err)
	}

	shop, err := s.activeShop(ctx, "id", p.ShopID)
	if err != nil {
		return Conversation{}, err
	}

	return s.start(ctx, buyerID, shop, sql.Null[int64]{V: p.ID, Valid: true})
}

// StartWithShop returns buyerID's conversation with the active shop at
// slug (no product), creating it on first use.
func (s *Service) StartWithShop(ctx context.Context, buyerID int64, slug string) (Conversation, error) {
	shop, err := s.activeShop(ctx, "slug", slug)
	if err != nil {
		return Conversation{}, err
	}

	return s.start(ctx, buyerID, shop, sql.Null[int64]{})
}

func (s *Service) activeShop(ctx context.Context, column string, v any) (catalog.Shop, error) {
	shop, err := orm.QueryOf[catalog.Shop](s.db).Where(column, v).Where("status", catalog.ShopActive).First(ctx)
	if errors.Is(err, orm.ErrNotFound) {
		return catalog.Shop{}, ErrNotFound
	}

	if err != nil {
		return catalog.Shop{}, fmt.Errorf("load shop: %w", err)
	}

	return shop, nil
}

func (s *Service) start(
	ctx context.Context,
	buyerID int64,
	shop catalog.Shop,
	product sql.Null[int64],
) (Conversation, error) {
	if shop.OwnerID == buyerID {
		return Conversation{}, ErrOwnShop
	}

	find := func() (Conversation, error) {
		q := orm.QueryOf[Conversation](s.db).Where("buyer_id", buyerID).Where("shop_id", shop.ID)
		if product.Valid {
			q = q.Where("product_id", product.V)
		} else {
			q = q.WhereNull("product_id")
		}

		return q.First(ctx)
	}

	c, err := find()
	if err == nil {
		return c, nil
	}

	if !errors.Is(err, orm.ErrNotFound) {
		return Conversation{}, fmt.Errorf("find conversation: %w", err)
	}

	c = Conversation{BuyerID: buyerID, ShopID: shop.ID, ProductID: product}
	if err := orm.Create(orm.WithClock(ctx, s.now), s.db, &c); err != nil {
		// Two tabs starting the same conversation: the unique index keeps
		// one; return it.
		if dbx.IsUniqueViolation(err) {
			return find()
		}

		return Conversation{}, fmt.Errorf("create conversation: %w", err)
	}

	return c, nil
}

// MessageView is a message as the pages and the broadcast events show it.
type MessageView struct {
	ID             int64     `json:"id"`
	ConversationID int64     `json:"conversation_id"`
	SenderID       int64     `json:"sender_id"`
	Body           string    `json:"body"`
	CreatedAt      time.Time `json:"created_at"`
}

func viewOf(m Message) MessageView {
	return MessageView{
		ID: m.ID, ConversationID: m.ConversationID, SenderID: m.SenderID, Body: m.Body, CreatedAt: m.CreatedAt,
	}
}

// InboxEvent tells a participant's open inbox (and header badge) that a
// conversation changed.
type InboxEvent struct {
	ConversationID int64       `json:"conversation_id"`
	Message        MessageView `json:"message"`
	// Unread is the recipient's total unread count.
	Unread int64 `json:"unread"`
}

// Send stores senderID's message in conversation id (body must have passed
// SendSchema), marks it read for the sender, broadcasts it and schedules
// the recipient's email. Broadcast and email failures are logged: the
// message is saved and clients catch up by id.
func (s *Service) Send(ctx context.Context, senderID, id int64, body string) (MessageView, error) {
	ctx = orm.WithClock(ctx, s.now)

	var (
		msg Message
		t   Thread
	)

	err := s.db.Tx(ctx, func(tx *database.Tx) error {
		var err error

		t, err = s.thread(ctx, tx, id, true)
		if err != nil {
			return err
		}

		role := t.Role(senderID)
		if role == "" {
			return ErrNotFound
		}

		msg = Message{ConversationID: id, SenderID: senderID, Body: body}
		if err := orm.Create(ctx, tx, &msg); err != nil {
			return fmt.Errorf("store message: %w", err)
		}

		_, err = dbx.Exec(ctx, tx, `UPDATE conversations SET last_message_id = ?, last_message_at = ?, `+
			readColumn(role)+` = ?, updated_at = ? WHERE id = ?`,
			msg.ID, msg.CreatedAt, msg.ID, msg.CreatedAt, id)

		return err
	})
	if err != nil {
		return MessageView{}, err
	}

	view := viewOf(msg)
	recipient := t.Counterpart(senderID)

	s.broadcast(ctx, MessagesChannel(id), EventMessage, view)

	unread, err := s.UnreadTotal(ctx, recipient)
	if err != nil {
		s.log.WarnContext(ctx, "chat: unread count failed", "err", err, "conversation_id", id)
	}

	s.broadcast(ctx, InboxChannel(recipient), EventInbox, InboxEvent{ConversationID: id, Message: view, Unread: unread})
	s.scheduleNotify(ctx, id, recipient)

	return view, nil
}

// readColumn is the read-receipt column of role (a fixed identifier).
func readColumn(role string) string {
	if role == RoleBuyer {
		return "buyer_last_read_id"
	}

	return "seller_last_read_id"
}

func (s *Service) broadcast(ctx context.Context, channel, event string, data any) {
	if s.publish == nil {
		return
	}

	if err := s.publish.Publish(ctx, channel, event, data); err != nil {
		s.log.WarnContext(ctx, "chat: broadcast failed", "err", err, "channel", channel, "event", event)
	}
}

// ReadEvent is a read receipt: UserID has read up to LastReadID.
type ReadEvent struct {
	UserID     int64 `json:"user_id"`
	LastReadID int64 `json:"last_read_id"`
}

// MarkRead moves userID's read receipt in conversation id to upTo (capped
// at the newest message; it never moves back) and tells the other side.
// It returns the receipt now stored.
func (s *Service) MarkRead(ctx context.Context, userID, id, upTo int64) (int64, error) {
	t, role, err := s.Participant(ctx, userID, id)
	if err != nil {
		return 0, err
	}

	target := min(upTo, t.LastMessageID)
	current := t.lastReadOf(role)

	if target <= current {
		return current, nil
	}

	n, err := dbx.Exec(ctx, s.db, `UPDATE conversations SET `+readColumn(role)+` = ? WHERE id = ? AND `+
		readColumn(role)+` < ?`, target, id, target)
	if err != nil {
		return 0, fmt.Errorf("mark conversation %d read: %w", id, err)
	}

	if n > 0 {
		s.broadcast(ctx, MessagesChannel(id), EventRead, ReadEvent{UserID: userID, LastReadID: target})
	}

	return target, nil
}

// TypingEvent says UserID is typing.
type TypingEvent struct {
	UserID int64 `json:"user_id"`
}

// Typing tells the other participant that userID is typing.
func (s *Service) Typing(ctx context.Context, userID, id int64) error {
	if _, _, err := s.Participant(ctx, userID, id); err != nil {
		return err
	}

	s.broadcast(ctx, MessagesChannel(id), EventTyping, TypingEvent{UserID: userID})

	return nil
}

// Since returns up to SinceLimit messages of conversation id newer than
// after, oldest first: the catch-up after a reconnect and the polling
// fallback.
func (s *Service) Since(ctx context.Context, userID, id, after int64) ([]MessageView, Thread, error) {
	t, _, err := s.Participant(ctx, userID, id)
	if err != nil {
		return nil, Thread{}, err
	}

	rows, err := orm.QueryOf[Message](s.db).Where("conversation_id", id).Where("id", ">", max(after, 0)).
		OrderBy("id", orm.Asc).Limit(SinceLimit).Get(ctx)
	if err != nil {
		return nil, Thread{}, fmt.Errorf("messages of conversation %d: %w", id, err)
	}

	return views(rows), t, nil
}

func views(rows []Message) []MessageView {
	out := make([]MessageView, 0, len(rows))
	for i := range rows {
		out = append(out, viewOf(rows[i]))
	}

	return out
}

// Recent returns the last History messages of conversation id, oldest
// first, without a participant check (callers checked it).
func (s *Service) Recent(ctx context.Context, id int64) ([]MessageView, error) {
	rows, err := orm.QueryOf[Message](s.db).Where("conversation_id", id).OrderBy("id", orm.Desc).
		Limit(History).Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("recent messages of conversation %d: %w", id, err)
	}

	out := views(rows)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}

	return out, nil
}

// UnreadTotal counts the messages userID has not read, over every
// conversation where they are the buyer or the shop owner.
func (s *Service) UnreadTotal(ctx context.Context, userID int64) (int64, error) {
	var n int64

	err := dbx.QueryRow(ctx, s.db, `SELECT COUNT(*) FROM chat_messages m
		JOIN conversations c ON c.id = m.conversation_id
		JOIN shops s ON s.id = c.shop_id
		WHERE m.sender_id <> ? AND (
			(c.buyer_id = ? AND m.id > c.buyer_last_read_id) OR
			(s.owner_id = ? AND m.id > c.seller_last_read_id))`,
		userID, userID, userID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count unread messages of user %d: %w", userID, err)
	}

	return n, nil
}

// FirstName is the first word of a display name ("" for an empty name).
func FirstName(name string) string {
	if f := strings.Fields(name); len(f) > 0 {
		return f[0]
	}

	return ""
}
