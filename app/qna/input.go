package qna

import "github.com/AlexGalhardo/vukem/z"

// Limits, in characters after trimming. resources/js/lib/qna.ts mirrors
// them.
const (
	QuestionMin = 10
	QuestionMax = 500
	AnswerMin   = 1
	AnswerMax   = 1000
	ReasonMin   = 3
	ReasonMax   = 500
)

// Validation and error messages that are i18n keys; the handlers
// translate them into the request locale.
const (
	MsgNoContact  = "qna.errors.no_contact"
	MsgOwnProduct = "qna.errors.own_product"
)

// noContact rejects text that moves the conversation off the platform.
func noContact(s string) error {
	if ContainsContact(s) {
		return z.FieldError("", MsgNoContact)
	}

	return nil
}

// AskInput is the question form on the product page.
type AskInput struct {
	Body string `json:"body" form:"body"`
}

// AskSchema validates AskInput.
var AskSchema = z.Struct[AskInput](z.Shape{
	"body": z.String().Trim().Min(QuestionMin).Max(QuestionMax).Refine(noContact),
})

// AnswerInput is the seller's answer form.
type AnswerInput struct {
	Answer string `json:"answer" form:"answer"`
}

// AnswerSchema validates AnswerInput.
var AnswerSchema = z.Struct[AnswerInput](z.Shape{
	"answer": z.String().Trim().Min(AnswerMin).Max(AnswerMax).Refine(noContact),
})

// HideInput is the admin's reason for hiding a question.
type HideInput struct {
	Reason string `json:"reason" form:"reason"`
}

// HideSchema validates HideInput.
var HideSchema = z.Struct[HideInput](z.Shape{
	"reason": z.String().Trim().Min(ReasonMin).Max(ReasonMax),
})
