package models

import "github.com/AlexGalhardo/vukem/orm"

// ContactMessage is a message sent through the public contact form; it is
// also forwarded to the ops Telegram chat.
type ContactMessage struct {
	orm.Model

	Name    string `json:"name"`
	Email   string `json:"email"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}
