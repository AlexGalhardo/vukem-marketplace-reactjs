package payments

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestFee(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		subtotal int64
		bps      int64
		want     int64
	}{
		{"ten percent", 10_000, 1000, 1000},
		{"rounds half up", 15, 1000, 2},         // 1.5 → 2
		{"rounds down below half", 14, 1000, 1}, // 1.4 → 1
		{"one cent", 1, 1000, 0},                // 0.1 → 0
		{"five cents", 5, 1000, 1},              // 0.5 → 1
		{"odd bps", 9_999, 1250, 1250},          // 1249.875 → 1250
		{"zero fee", 10_000, 0, 0},
		{"full fee", 10_000, 10_000, 10_000},
		{"zero subtotal", 0, 1000, 0},
		{"negative subtotal", -100, 1000, 0},
		{"large amount", 9_999_999_999, 1000, 1_000_000_000}, // 999999999.9 → 1000000000
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := Fee(tt.subtotal, tt.bps); got != tt.want {
				t.Errorf("Fee(%d, %d) = %d, want %d", tt.subtotal, tt.bps, got, tt.want)
			}
		})
	}
}

func TestFeeSplitOf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		subtotal, shipping int64
		want               Split
	}{
		{"fee excludes shipping", 10_000, 2_000, Split{10_000, 2_000, 1_000, 11_000}},
		{"rounded fee", 12_345, 1_590, Split{12_345, 1_590, 1_235, 12_700}},
		{"no shipping", 999, 0, Split{999, 0, 100, 899}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if diff := cmp.Diff(tt.want, SplitOf(tt.subtotal, tt.shipping, DefaultFeeBPS)); diff != "" {
				t.Errorf("SplitOf mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFeeValidateBPS(t *testing.T) {
	t.Parallel()

	for _, bps := range []int64{0, 1000, 10_000} {
		if err := ValidateFeeBPS(bps); err != nil {
			t.Errorf("ValidateFeeBPS(%d) = %v, want nil", bps, err)
		}
	}

	for _, bps := range []int64{-1, 10_001} {
		if err := ValidateFeeBPS(bps); err == nil {
			t.Errorf("ValidateFeeBPS(%d) = nil, want error", bps)
		}
	}
}
