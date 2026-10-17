package platform

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestDepsReady(t *testing.T) {
	t.Parallel()

	ok := func(context.Context) error { return nil }
	down := func(context.Context) error { return errors.New("connection refused") }

	tests := []struct {
		name    string
		checks  map[string]Check
		want    map[string]string
		wantErr bool
	}{
		{name: "no checks", checks: nil, want: map[string]string{}},
		{
			name:   "all ok",
			checks: map[string]Check{"db": ok, "redis": ok},
			want:   map[string]string{"db": "ok", "redis": "ok"},
		},
		{
			name:    "one down",
			checks:  map[string]Check{"db": ok, "mongo": down},
			want:    map[string]string{"db": "ok", "mongo": "fail"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var d Deps
			for name, c := range tt.checks {
				d.AddCheck(name, c)
			}

			got, err := d.Ready(t.Context())
			if (err != nil) != tt.wantErr {
				t.Fatalf("Ready() err = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && !errors.Is(err, ErrNotReady) {
				t.Fatalf("Ready() err = %v, want ErrNotReady", err)
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("Ready() checks (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDepsReadyTimesOut(t *testing.T) {
	t.Parallel()

	var d Deps
	d.AddCheck("hung", func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	got, err := d.Ready(ctx)
	if !errors.Is(err, context.Canceled) || got["hung"] != "fail" {
		t.Fatalf("Ready() = %v, %v; want hung=fail and context.Canceled", got, err)
	}
}
