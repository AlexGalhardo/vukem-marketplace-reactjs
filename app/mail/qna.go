package mail

import "github.com/AlexGalhardo/vukem/mail"

// NewQuestion tells a shop owner that a buyer asked about a product.
type NewQuestion struct {
	To           string
	OwnerName    string
	ProductTitle string
	Question     string
	// Link opens the seller's questions page.
	Link string
	// Subject is translated by the caller (Envelope has no context).
	Subject string
}

// Envelope implements mail.Mailable.
func (m NewQuestion) Envelope() mail.Envelope {
	return mail.Envelope{To: []string{m.To}, Subject: m.Subject}
}

// Content implements mail.Mailable.
func (m NewQuestion) Content() mail.Content { return mail.View("mail/new_question", m) }
