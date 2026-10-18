//go:build integration

package integration

import (
	"database/sql"
	"strconv"
	"testing"
	"time"

	"github.com/AlexGalhardo/vukem/orm"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/catalog"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/models"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/qna"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/internal/testing/authtest"
)

// TestQuestionsOnPostgres runs the questions flow on Postgres: the ask
// route, the demo insert the seeder uses, the public list order and
// pagination, the seller inbox and hiding.
func TestQuestionsOnPostgres(t *testing.T) {
	app, svc := catalogApp(t, nil)

	db, err := app.DB()
	if err != nil {
		t.Fatal(err)
	}

	owner := catalogSeller(t, svc, app, "SP")

	p, err := svc.CreateProduct(t.Context(), owner.ID, catalog.ProductInput{
		Title: "Batedeira " + word(8), Description: "x", PriceCents: 25000, Stock: 2, WeightGrams: 900,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := svc.SetProductStatus(t.Context(), owner.ID, p.ID, catalog.StatusPublished); err != nil {
		t.Fatal(err)
	}

	asker := models.User{
		Name:     "Iris Integração",
		Email:    word(12) + "@example.com",
		Password: "x",
		Role:     models.RoleUser,
	}
	// Verified: other integration tests may switch the email_verification
	// flag on in this shared database.
	asker.EmailVerifiedAt = orm.NullTime{V: time.Now().UTC(), Valid: true}
	if err := orm.Create(t.Context(), db, &asker); err != nil {
		t.Fatal(err)
	}

	base := time.Now().UTC().Add(-48 * time.Hour).Truncate(time.Second)
	rows := make([]qna.Question, 0, 12)

	for i := range 12 {
		r := qna.Question{
			CreatedAt: base.Add(time.Duration(i) * time.Hour),
			ProductID: p.ID, AskerID: asker.ID, Body: "Demo question number " + strconv.Itoa(i),
		}
		if i%2 == 0 {
			r.Answer = sql.Null[string]{V: "Demo answer " + strconv.Itoa(i), Valid: true}
			r.AnsweredAt = orm.NullTime{V: r.CreatedAt.Add(time.Hour), Valid: true}
		}

		rows = append(rows, r)
	}

	if err := qna.InsertDemo(t.Context(), db, rows); err != nil {
		t.Fatal(err)
	}

	authtest.ActingAs(t, app, app.Client(t).AcceptJSON(), asker).
		PostJSON("/p/"+p.Slug+"/questions", map[string]any{"body": "Does it come with two bowls?"}).
		AssertRedirect("/p/" + p.Slug)

	q, err := qna.New(qna.Config{DB: db})
	if err != nil {
		t.Fatal(err)
	}

	page, err := q.ProductQuestions(t.Context(), p.ID, 1)
	if err != nil {
		t.Fatal(err)
	}

	if page.Total != 13 || page.LastPage != 2 || page.Items[0].Body != "Does it come with two bowls?" ||
		page.Items[1].Body != "Demo question number 11" || page.Items[0].AskerName != "Iris" {
		t.Fatalf("product questions page 1 = %+v", page)
	}

	inbox, err := q.ShopQuestions(t.Context(), owner.ID, qna.FilterUnanswered, 1)
	if err != nil {
		t.Fatal(err)
	}

	if inbox.Total != 7 {
		t.Fatalf("unanswered = %d, want 7 (6 demo + 1 asked)", inbox.Total)
	}

	if err := q.Hide(t.Context(), owner.ID, inbox.Items[0].ID, "Integration check"); err != nil {
		t.Fatal(err)
	}

	if n, err := q.UnansweredCount(t.Context(), owner.ID); err != nil || n != 6 {
		t.Fatalf("unanswered after hiding = %d, %v; want 6", n, err)
	}

	answered, err := q.ShopQuestions(t.Context(), owner.ID, qna.FilterAnswered, 1)
	if err != nil || answered.Total != 6 || answered.Items[0].AnsweredAt == nil {
		t.Fatalf("answered = %+v, %v", answered, err)
	}
}
