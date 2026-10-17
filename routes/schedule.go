package routes

import (
	"context"
	"time"

	"github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/schedule"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/maintenance"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/reviews"
)

// Schedule declares the scheduled tasks. Run them with "vukem schedule:work"
// (the compose "scheduler" service, or inside serve with SCHEDULE_IN_SERVE)
// or a cron entry calling "vukem schedule:run" every minute; list them with
// "vukem schedule:list". WithoutOverlapping locks live in the app cache
// (Redis when REDIS_URL is set), so several schedulers never run the same
// task twice.
func Schedule(s *schedule.Schedule, app *vukem.App) {
	// Unpaid PIX orders past their deadline: pending → expired.
	s.Func("payments:expire-pix", func(ctx context.Context) error {
		pay, err := NewPayments(app)
		if err != nil {
			return err
		}

		n, err := pay.ExpirePix(ctx)
		if n > 0 {
			app.Log.InfoContext(ctx, "expired PIX orders", "count", n)
		}

		return err
	}).EveryMinute().WithoutOverlapping()

	db, err := app.DB()
	if err != nil {
		// Surface the error on every run instead of dropping the tasks.
		fail := func(context.Context) error { return err }
		s.Func("maintenance:prune-tokens", fail).Hourly()

		return
	}

	// Same TTL as accounts.Accounts.ResetTTL (password reset links work for an hour).
	p := maintenance.Pruner{DB: db, ResetTTL: time.Hour, Now: app.Now}

	s.Func("maintenance:prune-tokens", p.Tokens).HourlyAt(5).WithoutOverlapping()
	s.Func("maintenance:prune-provider-events", p.ProviderEvents).DailyAt("03:15").WithoutOverlapping()
	s.Func("maintenance:prune-webhook-deliveries", p.WebhookDeliveries).DailyAt("03:30").WithoutOverlapping()
	// Ratings change in the review transactions; this rebuild only repairs
	// drift from manual SQL or batch imports.
	s.Func("reviews:recalculate-ratings", func(ctx context.Context) error { return reviews.Recalculate(ctx, db) }).
		DailyAt("04:10").WithoutOverlapping()
}
