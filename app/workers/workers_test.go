package workers_test

import (
	"context"
	"testing"
	"time"

	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/queue"
	"github.com/AlexGalhardo/vukem/vukemtest"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/workers"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/bootstrap"
)

type ping struct{ N int }

func (ping) Kind() string { return "test.ping" }

// persistent stands in for the Redis driver: a stored queue that vukem
// does not work by itself (it only does so for *queue.Memory).
type persistent struct{ *queue.Memory }

func TestQueueRunnerProcessesJobsInProcess(t *testing.T) {
	t.Parallel()

	done := make(chan int, 1)
	app := vukemtest.New(t, bootstrap.New, vukemtest.Options{AppOptions: []vukem.Option{
		vukem.WithQueueDriver(persistent{queue.NewMemory()}),
		vukem.WithJobs(queue.Handle(func(_ context.Context, j ping) error {
			done <- j.N
			return nil
		})),
	}})

	q, err := app.Queue()
	if err != nil {
		t.Fatal(err)
	}

	if err := q.Dispatch(t.Context(), ping{N: 7}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	stopped := make(chan error, 1)

	go func() { stopped <- workers.QueueRunner(app.App).Run(ctx) }()

	select {
	case n := <-done:
		if n != 7 {
			t.Errorf("job payload %d, want 7", n)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the in-process worker did not run the queued job")
	}

	cancel()

	if err := <-stopped; err != nil {
		t.Errorf("runner: %v", err)
	}
}

func TestQueueRunnerSkipsInlineQueue(t *testing.T) {
	t.Parallel()

	// The default test queue is sync: the runner must return at once
	// instead of polling a driver that never stores jobs.
	app := vukemtest.New(t, bootstrap.New)
	if err := workers.QueueRunner(app.App).Run(t.Context()); err != nil {
		t.Fatal(err)
	}
}
