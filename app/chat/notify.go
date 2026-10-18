package chat

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/AlexGalhardo/vukem/i18n"
	"github.com/AlexGalhardo/vukem/orm"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/catalog"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/dbx"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/models"
)

// NotifyJob checks, NotifyAfter after a message, whether its recipient is
// still away with unread messages in the conversation and emails them.
type NotifyJob struct {
	ConversationID int64  `json:"conversation_id"`
	RecipientID    int64  `json:"recipient_id"`
	Locale         string `json:"locale"`
}

// Kind implements queue.Job.
func (NotifyJob) Kind() string { return "chat.notify" }

// NotifyMail is what the email needs; Config.Mail renders and queues it.
type NotifyMail struct {
	To             models.User
	From           string // the other side: shop name or buyer first name
	Unread         int64
	Preview        string
	ConversationID int64
	Locale         string
}

// previewMax caps the message excerpt in the email, in characters.
const previewMax = 140

func claimKey(conversationID, userID int64) string {
	return "chat:notify:" + strconv.FormatInt(conversationID, 10) + ":" + strconv.FormatInt(userID, 10)
}

// scheduleNotify dispatches one delayed NotifyJob per conversation and
// recipient per NotifyAfter window: the first message claims the window
// (an atomic cache increment), later ones ride on the same job, so a burst
// of messages sends at most one email.
func (s *Service) scheduleNotify(ctx context.Context, conversationID, recipient int64) {
	if s.notify == nil {
		return
	}

	n, err := s.cache.Increment(ctx, claimKey(conversationID, recipient), 1, s.notifyAfter)
	if err != nil {
		s.log.WarnContext(ctx, "chat: notification claim failed", "err", err, "conversation_id", conversationID)
		return
	}

	if n != 1 {
		return
	}

	job := NotifyJob{ConversationID: conversationID, RecipientID: recipient, Locale: i18n.Locale(ctx)}
	if err := s.notify(ctx, job, s.notifyAfter); err != nil {
		s.log.WarnContext(ctx, "chat: notification not queued", "err", err, "conversation_id", conversationID)
	}
}

// NotifyAfter is the away time before an unread message is emailed.
func (s *Service) NotifyAfter() time.Duration { return s.notifyAfter }

// HandleNotify runs NotifyJob: it emails the recipient when they still are
// a participant, still have unread messages here and have not been seen
// for NotifyAfter. It returns whether an email was queued.
func (s *Service) HandleNotify(
	ctx context.Context,
	job NotifyJob,
	mail func(context.Context, NotifyMail) error,
) (bool, error) {
	t, role, err := s.Participant(ctx, job.RecipientID, job.ConversationID)
	if err != nil {
		return false, nil //nolint:nilerr // the conversation or the participant is gone: nothing to send
	}

	if at, ok, err := s.LastSeen(ctx, job.RecipientID); err != nil {
		return false, err
	} else if ok && s.now().Sub(at) < s.notifyAfter {
		return false, nil
	}

	var (
		unread  int64
		preview string
	)

	err = dbx.QueryRow(ctx, s.db, `SELECT COUNT(*) FROM chat_messages
		WHERE conversation_id = ? AND sender_id <> ? AND id > ?`,
		t.ID, job.RecipientID, t.lastReadOf(role)).Scan(&unread)
	if err != nil {
		return false, fmt.Errorf("chat: unread of conversation %d: %w", t.ID, err)
	}

	if unread == 0 {
		return false, nil
	}

	last, err := orm.QueryOf[Message](s.db).Where("conversation_id", t.ID).Where("sender_id", "<>", job.RecipientID).
		OrderBy("id", orm.Desc).First(ctx)
	if err != nil {
		return false, fmt.Errorf("chat: last message of conversation %d: %w", t.ID, err)
	}

	if r := []rune(last.Body); len(r) > previewMax {
		preview = string(r[:previewMax]) + "…"
	} else {
		preview = last.Body
	}

	to, err := orm.QueryOf[models.User](s.db).Find(ctx, job.RecipientID)
	if err != nil {
		return false, fmt.Errorf("chat: load recipient %d: %w", job.RecipientID, err)
	}

	from, err := s.otherName(ctx, t, role)
	if err != nil {
		return false, err
	}

	err = mail(ctx, NotifyMail{
		To: to, From: from, Unread: unread, Preview: preview, ConversationID: t.ID, Locale: job.Locale,
	})
	if err != nil {
		return false, fmt.Errorf("chat: queue notification mail: %w", err)
	}

	return true, nil
}

// otherName is how role sees the other side: the shop's name for the
// buyer, the buyer's first name for the seller.
func (s *Service) otherName(ctx context.Context, t Thread, role string) (string, error) {
	if role == RoleBuyer {
		shop, err := orm.QueryOf[catalog.Shop](s.db).Select("id", "name").Find(ctx, t.ShopID)
		if err != nil {
			return "", fmt.Errorf("chat: load shop %d: %w", t.ShopID, err)
		}

		return shop.Name, nil
	}

	buyer, err := orm.QueryOf[models.User](s.db).Select("id", "name").Find(ctx, t.BuyerID)
	if err != nil {
		return "", fmt.Errorf("chat: load buyer %d: %w", t.BuyerID, err)
	}

	return FirstName(buyer.Name), nil
}
