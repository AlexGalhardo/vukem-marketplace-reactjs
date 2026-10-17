package routes

import (
	"context"

	"github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/schedule"
)

// Schedule declares the scheduled tasks. Run them with "vukem schedule:work"
// or a cron entry calling "vukem schedule:run" every minute; list them with
// "vukem schedule:list". Examples:
//
//	s.Command("reports:send").DailyAt("08:00").WithoutOverlapping()
//	s.Job(jobs.PruneSessions{}).Hourly()
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
}
