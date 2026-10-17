package payments

import "testing"

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	good := map[string]string{
		"STRIPE_SECRET_KEY":         "sk_test_1",
		"STRIPE_PUBLISHABLE_KEY":    "pk_test_1",
		"STRIPE_WEBHOOK_SECRETS":    "whsec_a, whsec_b",
		"ABACATEPAY_API_KEY":        "abc_dev_1",
		"ABACATEPAY_WEBHOOK_SECRET": "hook-secret",
	}

	with := func(over map[string]string) func(string) string {
		return func(k string) string {
			if v, ok := over[k]; ok {
				return v
			}

			return good[k]
		}
	}

	none := func(string) string { return "" }

	tests := []struct {
		name       string
		get        func(string) string
		production bool
		wantErr    bool
		card, pix  bool
		bps        int64
	}{
		{"nothing set boots with providers off", none, true, false, false, false, DefaultFeeBPS},
		{"all set", with(nil), true, false, true, true, DefaultFeeBPS},
		{"custom fee", with(map[string]string{"PLATFORM_FEE_BPS": "1250"}), true, false, true, true, 1250},
		{
			"bad fee is always an error",
			with(map[string]string{"PLATFORM_FEE_BPS": "abc"}),
			false,
			true,
			false,
			false,
			0,
		},
		{"fee above 100%", with(map[string]string{"PLATFORM_FEE_BPS": "10001"}), false, true, false, false, 0},
		{
			"publishable key as secret in production",
			with(map[string]string{"STRIPE_SECRET_KEY": "pk_test_1"}),
			true,
			true,
			false,
			false,
			0,
		},
		{
			"publishable key as secret in dev disables card",
			with(map[string]string{"STRIPE_SECRET_KEY": "pk_test_1"}),
			false,
			false,
			false,
			true,
			DefaultFeeBPS,
		},
		{
			"key without webhook secret in production",
			with(map[string]string{"STRIPE_WEBHOOK_SECRETS": ""}),
			true,
			true,
			false,
			false,
			0,
		},
		{
			"bad webhook secret",
			with(map[string]string{"STRIPE_WEBHOOK_SECRETS": "whsec_a,oops"}),
			true,
			true,
			false,
			false,
			0,
		},
		{
			"pix key without webhook secret",
			with(map[string]string{"ABACATEPAY_WEBHOOK_SECRET": ""}),
			true,
			true,
			false,
			false,
			0,
		},
		{
			"pix half-configured in dev disables pix",
			with(map[string]string{"ABACATEPAY_API_KEY": ""}),
			false,
			false,
			true,
			false,
			DefaultFeeBPS,
		},
		{
			"plain http base url in production",
			with(map[string]string{"ABACATEPAY_BASE_URL": "http://api.abacatepay.com"}),
			true,
			true,
			false,
			false,
			0,
		},
		{
			"http base url allowed in dev",
			with(map[string]string{"ABACATEPAY_BASE_URL": "http://localhost:9"}),
			false,
			false,
			true,
			true,
			DefaultFeeBPS,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, err := LoadConfig(tt.get, tt.production, false)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}

			if err != nil {
				return
			}

			if c.CardEnabled() != tt.card || c.PixEnabled() != tt.pix || c.FeeBPS != tt.bps {
				t.Errorf(
					"card=%v pix=%v bps=%d; want %v %v %d",
					c.CardEnabled(),
					c.PixEnabled(),
					c.FeeBPS,
					tt.card,
					tt.pix,
					tt.bps,
				)
			}
		})
	}
}

func TestLoadConfigSplitsWebhookSecrets(t *testing.T) {
	t.Parallel()

	c, err := LoadConfig(func(k string) string {
		return map[string]string{
			"STRIPE_SECRET_KEY": "sk_test_1", "STRIPE_WEBHOOK_SECRETS": "whsec_a,,whsec_b ",
			"STRIPE_API_BASE": "http://127.0.0.1:1",
		}[k]
	}, false, false)
	if err != nil {
		t.Fatal(err)
	}

	if len(c.StripeWebhookSecrets) != 2 || c.StripeWebhookSecrets[1].Reveal() != "whsec_b" {
		t.Errorf("secrets = %d", len(c.StripeWebhookSecrets))
	}

	if c.StripeAPIBase != "" {
		t.Error("STRIPE_API_BASE honoured outside tests")
	}
}

func TestAbacatePaySignature(t *testing.T) {
	t.Parallel()

	body := []byte(`{"id":"evt_1","event":"transparent.completed"}`)
	sig := SignAbacatePayForTest(body, abacatePayPublicKey)

	if !ValidAbacatePaySignature(body, sig, abacatePayPublicKey) {
		t.Error("valid signature rejected")
	}

	for _, bad := range []string{"", sig + "x", SignAbacatePayForTest(body, "other")} {
		if ValidAbacatePaySignature(body, bad, abacatePayPublicKey) {
			t.Errorf("signature %q accepted", bad)
		}
	}

	if ValidAbacatePaySignature(append(body, ' '), sig, abacatePayPublicKey) {
		t.Error("tampered body accepted")
	}
}
