package requests

import "github.com/AlexGalhardo/vukem/z"

// Contact is the public contact form.
type Contact struct {
	Name    string `json:"name"    form:"name"`
	Email   string `json:"email"   form:"email"`
	Subject string `json:"subject" form:"subject"`
	Body    string `json:"body"    form:"body"`
}

// ContactSchema validates Contact.
var ContactSchema = z.Struct[Contact](z.Shape{
	"name":    z.String().Trim().Min(2).Max(100),
	"email":   z.String().Trim().Email().Max(255),
	"subject": z.String().Trim().Min(3).Max(150),
	"body":    z.String().Trim().Min(10).Max(5000),
})

// WebhookEndpoint is the seller's new webhook endpoint form.
type WebhookEndpoint struct {
	URL    string   `json:"url"    form:"url"`
	Events []string `json:"events" form:"events"`
}

// WebhookEndpointSchema validates WebhookEndpoint; the URL's safety and
// the event names are checked by the webhooks service.
var WebhookEndpointSchema = z.Struct[WebhookEndpoint](z.Shape{
	"url":    z.String().Trim().Min(1).Max(2048),
	"events": z.Slice(z.String().Max(64)).Max(10),
})

// ClientLog is a browser error report (see controllers.ClientLog).
type ClientLog struct {
	Level     string `json:"level"`
	Message   string `json:"message"`
	Stack     string `json:"stack,omitempty"`
	URL       string `json:"url,omitempty"`
	UserAgent string `json:"userAgent,omitempty"`
}

// ClientLogSchema validates ClientLog; every field has a hard cap because
// the endpoint is public.
var ClientLogSchema = z.Struct[ClientLog](z.Shape{
	"level":     z.String().OneOf("warn", "error", "fatal"),
	"message":   z.String().Trim().Min(1).Max(500),
	"stack":     z.String().Max(2000).Optional(),
	"url":       z.String().Max(300).Optional(),
	"userAgent": z.String().Max(200).Optional(),
})
