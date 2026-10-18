package mail

import "github.com/AlexGalhardo/vukem/mail"

// ChatMessage tells a chat participant who has been away that the other
// side wrote to them.
type ChatMessage struct {
	To   string
	Name string
	// From is the other side (a shop name or a buyer's first name).
	From    string
	Unread  int64
	Preview string
	// Link opens the conversation.
	Link string
	// Subject is translated by the caller (Envelope has no context).
	Subject string
}

// Envelope implements mail.Mailable.
func (m ChatMessage) Envelope() mail.Envelope {
	return mail.Envelope{To: []string{m.To}, Subject: m.Subject}
}

// Content implements mail.Mailable.
func (m ChatMessage) Content() mail.Content { return mail.View("mail/chat_message", m) }
