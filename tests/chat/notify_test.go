package chat_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/queue"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/chat"
)

func (e *env) jobs(kind string) []queue.Message {
	var out []queue.Message

	msgs := e.queue.Messages()
	for i := range msgs {
		if msgs[i].Kind == kind {
			out = append(out, msgs[i])
		}
	}

	return out
}

// process runs one recorded job through the app's queue, like a worker.
func (e *env) process(t *testing.T, m queue.Message) {
	t.Helper()

	q, err := e.app.Queue()
	if err != nil {
		t.Fatal(err)
	}

	if err := q.Process(t.Context(), &m); err != nil {
		t.Fatalf("job %s: %v", m.Kind, err)
	}
}

func TestOfflineRecipientIsEmailedOncePerWindow(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	buyer := e.user(t, "Bia Buyer")
	id := e.start(t, buyer)

	before := time.Now()

	e.send(t, buyer, id, "Primeira mensagem")
	e.send(t, buyer, id, "Segunda mensagem")
	e.send(t, buyer, id, "Terceira mensagem")

	// A burst of messages schedules one delayed check for the seller (the
	// buyer, who sends, is around).
	jobs := e.jobs("chat.notify")
	if len(jobs) != 1 {
		t.Fatalf("%d notify jobs, want 1 for the burst", len(jobs))
	}

	if delay := jobs[0].AvailableAt.Sub(before); delay < 9*time.Minute || delay > 11*time.Minute {
		t.Fatalf("notify job runs after %s, want about 10m (CHAT_NOTIFY_AFTER default)", delay)
	}

	var job chat.NotifyJob
	if err := json.Unmarshal(jobs[0].Payload, &job); err != nil {
		t.Fatal(err)
	}

	if job.RecipientID != e.owner.ID || job.ConversationID != id {
		t.Fatalf("job %+v, want the seller of conversation %d", job, id)
	}

	// The seller never came back: the job queues one email.
	e.process(t, jobs[0])

	if n := mailJobs(t, e, e.owner.Email); n != 1 {
		t.Fatalf("%d mail jobs, want 1", n)
	}
}

func TestNotifySkipsRecipientsWhoAreAroundOrHaveRead(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	buyer := e.user(t, "Bia Buyer")
	id := e.start(t, buyer)
	msg := e.send(t, buyer, id, "Olá, tudo bem?")

	now := time.Now()
	svc := e.service(t, func() time.Time { return now })
	job := chat.NotifyJob{ConversationID: id, RecipientID: e.owner.ID}

	var sent []chat.NotifyMail

	mail := func(_ context.Context, m chat.NotifyMail) error {
		sent = append(sent, m)
		return nil
	}

	// Seen 3 minutes ago: not away for 10 minutes yet.
	at := now.Add(-3 * time.Minute)
	svc2 := e.service(t, func() time.Time { return at })
	svc2.Touch(t.Context(), e.owner.ID)

	if ok, err := svc.HandleNotify(t.Context(), job, mail); err != nil || ok {
		t.Fatalf("recently seen: sent=%v err=%v, want no email", ok, err)
	}

	// Away for 11 minutes: emailed, with the shop's view of the buyer.
	at = now.Add(-11 * time.Minute)
	svc2.Touch(t.Context(), e.owner.ID)

	if ok, err := svc.HandleNotify(t.Context(), job, mail); err != nil || !ok {
		t.Fatalf("away: sent=%v err=%v, want an email", ok, err)
	}

	if len(sent) != 1 || sent[0].From != "Bia" || sent[0].Unread != 1 || sent[0].Preview != "Olá, tudo bem?" {
		t.Fatalf("mail %+v", sent)
	}

	// Read in the meantime: nothing to tell.
	e.json(t, e.owner).PostJSON(path(id, "/read"), map[string]any{"last_id": msg}).AssertOK()

	if ok, err := svc.HandleNotify(t.Context(), job, mail); err != nil || ok {
		t.Fatalf("after reading: sent=%v err=%v, want no email", ok, err)
	}

	// Not a participant (a forged job): nothing.
	stranger := e.user(t, "Stranger")
	if ok, err := svc.HandleNotify(t.Context(), chat.NotifyJob{ConversationID: id, RecipientID: stranger.ID},
		mail); err != nil || ok {
		t.Fatalf("stranger: sent=%v err=%v, want no email", ok, err)
	}
}

func TestNotifyAfterIsConfigurable(t *testing.T) {
	t.Parallel()

	e := newEnv(t, map[string]string{"CHAT_NOTIFY_AFTER": "2m"})
	buyer := e.user(t, "Bia Buyer")
	id := e.start(t, buyer)

	before := time.Now()
	e.send(t, buyer, id, "Oi")

	jobs := e.jobs("chat.notify")
	if len(jobs) != 1 {
		t.Fatalf("%d notify jobs, want 1", len(jobs))
	}

	if delay := jobs[0].AvailableAt.Sub(before); delay < time.Minute || delay > 3*time.Minute {
		t.Fatalf("delay %s, want about 2m", delay)
	}
}

// mailJobs counts the queued mail jobs; their payload is sealed, so the
// recipient must not appear in clear text.
func mailJobs(t *testing.T, e *env, email string) int {
	t.Helper()

	n := 0

	jobs := e.jobs(vukem.MailJobKind)
	for i := range jobs {
		if strings.Contains(string(jobs[i].Payload), email) {
			t.Fatalf("queued mail payload shows %s in clear text", email)
		}

		n++
	}

	return n
}
