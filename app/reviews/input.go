package reviews

import (
	"github.com/AlexGalhardo/vukem/z"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/moderation"
)

// MsgContact is the error for text carrying a link, an email address or a
// phone number (moderation.ContainsContact); the pages translate it.
const MsgContact = "reviews.errors.contact"

func noContact(s string) error {
	if moderation.ContainsContact(s) {
		return z.FieldError("", MsgContact)
	}

	return nil
}

// InputSchema validates a review: 1 to 5 stars, an optional title and
// text, an optional "would recommend" (yes, no or empty), no contact data.
var InputSchema = z.Struct[Input](z.Shape{
	"rating":    z.Int().Min(1).Max(5),
	"title":     z.String().Trim().Max(MaxTitle).Refine(noContact).Optional(),
	"body":      z.String().Trim().Max(MaxBody).Refine(noContact).Optional(),
	"recommend": z.String().OneOf("", "yes", "no").Optional(),
})

// ReplyInput is the shop's answer to a review.
type ReplyInput struct {
	Body string `json:"body" form:"body"`
}

// ReplySchema validates ReplyInput: 1 to MaxReply characters, no contact
// data.
var ReplySchema = z.Struct[ReplyInput](z.Shape{
	"body": z.String().Trim().Min(1).Max(MaxReply).Refine(noContact),
})

// HideInput is the admin's reason to hide a review.
type HideInput struct {
	Reason string `json:"reason" form:"reason"`
}

// HideSchema validates HideInput.
var HideSchema = z.Struct[HideInput](z.Shape{
	"reason": z.String().Trim().Min(MinReason).Max(MaxReason),
})
