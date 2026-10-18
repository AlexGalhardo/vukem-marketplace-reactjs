package chat

import (
	"github.com/AlexGalhardo/vukem/z"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/moderation"
)

// Message limits, in characters after trimming. resources/js/lib/chat.ts
// mirrors them.
const (
	MessageMin = 1
	MessageMax = 2000
)

// Validation and error messages that are i18n keys; the handlers
// translate them into the request locale.
const (
	MsgNoContact = "chat.errors.no_contact"
	MsgOwnShop   = "chat.errors.own_shop"
)

func noContact(s string) error {
	if moderation.ContainsContact(s) {
		return z.FieldError("", MsgNoContact)
	}

	return nil
}

// SendInput is one chat message. The body is plain text: it is stored as
// typed and the pages render it as text, never as HTML.
type SendInput struct {
	Body string `json:"body" form:"body"`
}

// SendSchema validates SendInput.
var SendSchema = z.Struct[SendInput](z.Shape{
	"body": z.String().Trim().Min(MessageMin).Max(MessageMax).Refine(noContact),
})

// StartInput opens a conversation about a product (its slug) or with a
// shop (its slug, e.g. from an order); exactly one is set.
type StartInput struct {
	Product string `json:"product" form:"product"`
	Shop    string `json:"shop"    form:"shop"`
}

// StartSchema validates StartInput.
var StartSchema = z.Struct[StartInput](z.Shape{
	"product": z.String().Trim().Max(200).Optional(),
	"shop":    z.String().Trim().Max(200).Optional(),
})

// ReadInput marks the conversation read up to LastID.
type ReadInput struct {
	LastID int64 `json:"last_id" form:"last_id"`
}

// ReadSchema validates ReadInput.
var ReadSchema = z.Struct[ReadInput](z.Shape{
	"last_id": z.Int64().Min(1),
})
