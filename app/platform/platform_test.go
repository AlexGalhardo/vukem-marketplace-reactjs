package platform

import (
	"context"
	"errors"
	"testing"

	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/vukemtest"
)

func TestDepsChecksReachTheAppBeforeAndAfterBind(t *testing.T) {
	t.Parallel()

	app := vukemtest.New(t, vukem.New)
	down := errors.New("connection refused")

	d := &Deps{}
	d.AddCheck("early", func(context.Context) error { return nil })
	d.Bind(app.App)
	d.AddCheck("late", func(context.Context) error { return down })

	got := app.CheckHealth(t.Context())

	if err, ok := got["early"]; !ok || err != nil {
		t.Errorf("early check = %v (registered %v), want ok", err, ok)
	}

	if err := got["late"]; !errors.Is(err, down) {
		t.Errorf("late check = %v, want %v", err, down)
	}
}
