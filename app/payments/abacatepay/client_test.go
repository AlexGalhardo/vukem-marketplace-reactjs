package abacatepay_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/AlexGalhardo/vukem/config"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/payments"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/payments/abacatepay"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/payments/abacatepay/abacatepaytest"
)

func TestCreatePixChargeAndSimulate(t *testing.T) {
	t.Parallel()

	srv := abacatepaytest.New(t, "abc_dev_key")

	c, err := abacatepay.New(config.NewSecret("abc_dev_key"), srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	params := payments.PixChargeParams{
		AmountCents: 12_345,
		ExternalID:  "0190aaaa-0000-7000-8000-000000000001",
		Description: "Order",
		ExpiresIn:   30 * time.Minute,
	}

	ch, err := c.CreatePixCharge(t.Context(), params)
	if err != nil {
		t.Fatal(err)
	}

	if ch.ID == "" || ch.BRCode == "" || !strings.HasPrefix(ch.QRImage, "data:image/png;base64,") ||
		ch.ExpiresAt.IsZero() {
		t.Errorf("charge = %+v", ch)
	}

	// externalId makes the create idempotent.
	again, err := c.CreatePixCharge(t.Context(), params)
	if err != nil || again.ID != ch.ID {
		t.Errorf("second create = %q (%v), want %q", again.ID, err, ch.ID)
	}

	got := srv.Charges()
	if len(got) != 1 || got[0].ExpiresIn != 1800 || got[0].Amount != 12_345 {
		t.Errorf("fake charges = %+v", got)
	}

	if err := c.SimulatePayment(t.Context(), ch.ID); err != nil {
		t.Fatal(err)
	}

	if !srv.Charges()[0].Paid {
		t.Error("simulate-payment did not pay the charge")
	}

	if err := c.SimulatePayment(t.Context(), "unknown"); !errors.Is(err, abacatepay.ErrAPI) {
		t.Errorf("simulate unknown err = %v", err)
	}
}

func TestClientErrors(t *testing.T) {
	t.Parallel()

	srv := abacatepaytest.New(t, "right")

	wrong, _ := abacatepay.New(config.NewSecret("wrong"), srv.URL)
	if _, err := wrong.CreatePixCharge(
		t.Context(),
		payments.PixChargeParams{AmountCents: 500},
	); !errors.Is(
		err,
		abacatepay.ErrAPI,
	) {
		t.Errorf("wrong key err = %v", err)
	}

	if _, err := abacatepay.New(config.Secret{}, srv.URL); err == nil {
		t.Error("New without key succeeded")
	}

	srv.SetFailing(true)

	right, _ := abacatepay.New(config.NewSecret("right"), srv.URL)
	if _, err := right.CreatePixCharge(
		t.Context(),
		payments.PixChargeParams{AmountCents: 500},
	); !errors.Is(
		err,
		abacatepay.ErrAPI,
	) {
		t.Errorf("server error err = %v", err)
	}
}

func TestQRDataURIOnlyAcceptsBase64Images(t *testing.T) {
	t.Parallel()

	tests := []struct{ in, want string }{
		{"iVBORw0KGgo=", "data:image/png;base64,iVBORw0KGgo="},
		{"data:image/png;base64,iVBORw0KGgo=", "data:image/png;base64,iVBORw0KGgo="},
		{"javascript:alert(1)", ""},
		{`"><script>alert(1)</script>`, ""},
		{"data:text/html;base64,PHNjcmlwdD4=", ""},
		{"", ""},
	}

	for _, tt := range tests {
		if got := abacatepay.QRDataURI(tt.in); got != tt.want {
			t.Errorf("QRDataURI(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
