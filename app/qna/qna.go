// Package qna is the product questions and answers service. Buyers ask on
// the product page, the shop owner answers from the seller area and admins
// hide abusive questions. Rules enforced here and nowhere else:
//
//   - only visible products (published, shop active) take questions;
//   - a shop owner cannot ask on their own shop's products;
//   - seller methods take the acting user id and answer ErrNotFound for
//     questions of other shops (never 403, so ids do not leak);
//   - hidden questions never reach the public list or the seller inbox;
//   - questions and answers carry no contact data (ContainsContact).
package qna

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/errs"
	"github.com/AlexGalhardo/vukem/orm"

	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/catalog"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/models"
	"github.com/AlexGalhardo/vukem-micro-mercadolivre-reactjs/app/realtime"
)

var (
	// ErrNotFound: the product or question does not exist, is not visible
	// or belongs to another shop (404).
	ErrNotFound = catalog.ErrNotFound
	// ErrOwnProduct: a shop owner asked on their own product (403). The
	// message is an i18n key.
	ErrOwnProduct = errs.New(http.StatusForbidden, MsgOwnProduct)
	// ErrNoShop: the acting user has no shop.
	ErrNoShop = catalog.ErrNoShop
)

// PerPage is the page size of every question list.
const PerPage = 10

// Question is one row of product_questions.
type Question struct {
	orm.Model

	ProductID    int64            `db:"product_id"`
	ShopID       int64            `db:"shop_id"`
	AskerID      int64            `db:"asker_id"`
	Body         string           `db:"body"`
	Answer       sql.Null[string] `db:"answer"`
	AnsweredAt   orm.NullTime     `db:"answered_at"`
	HiddenAt     orm.NullTime     `db:"hidden_at"`
	HiddenBy     sql.Null[int64]  `db:"hidden_by"`
	HiddenReason sql.Null[string] `db:"hidden_reason"`
}

// Table implements orm.Tabler.
func (Question) Table() string { return "product_questions" }

// Notifier publishes realtime events to one user (realtime.Hub).
type Notifier interface {
	Publish(ctx context.Context, userID int64, channel string, payload any) error
}

// NewQuestionMail is what the owner's email needs; Config.MailOwner
// renders and queues it.
type NewQuestionMail struct {
	Owner        models.User
	ProductTitle string
	ProductSlug  string
	Question     string
}

// Config holds the service dependencies. Notify and MailOwner are
// optional (nil skips them).
type Config struct {
	DB     *database.DB
	Notify Notifier
	// MailOwner queues the new-question email to the shop owner.
	MailOwner func(ctx context.Context, m NewQuestionMail) error
	Log       *slog.Logger
	Now       func() time.Time
}

// Service is the questions API used by the controllers and the seeders.
type Service struct {
	db        *database.DB
	notify    Notifier
	mailOwner func(ctx context.Context, m NewQuestionMail) error
	log       *slog.Logger
	now       func() time.Time
}

// New returns the service.
func New(cfg Config) (*Service, error) {
	if cfg.DB == nil {
		return nil, errors.New("qna: Config.DB is required")
	}

	s := &Service{db: cfg.DB, notify: cfg.Notify, mailOwner: cfg.MailOwner, log: cfg.Log, now: cfg.Now}
	if s.log == nil {
		s.log = slog.New(slog.DiscardHandler)
	}

	if s.now == nil {
		s.now = time.Now
	}

	return s, nil
}

// ProductRef names the product a question is about.
type ProductRef struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

// PublicQuestion is a question as the product page shows it. The asker is
// shown by first name only.
type PublicQuestion struct {
	ID         int64      `json:"id"`
	Body       string     `json:"body"`
	Answer     string     `json:"answer"`
	AnsweredAt *time.Time `json:"answered_at"`
	AskerName  string     `json:"asker_name"`
	CreatedAt  time.Time  `json:"created_at"`
}

// SellerQuestion is a question in the seller inbox.
type SellerQuestion struct {
	PublicQuestion

	Product ProductRef `json:"product"`
}

// AdminQuestion is a question on the moderation page.
type AdminQuestion struct {
	SellerQuestion

	AskerID      int64      `json:"asker_id"`
	ShopName     string     `json:"shop_name"`
	HiddenAt     *time.Time `json:"hidden_at"`
	HiddenReason string     `json:"hidden_reason"`
}

// Page is one page of results.
type Page[T any] = catalog.Page[T]

// FirstName is the first word of a display name ("" for an empty name).
func FirstName(name string) string {
	if f := strings.Fields(name); len(f) > 0 {
		return f[0]
	}

	return ""
}

func timePtr(t orm.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}

	return &t.V
}

// visibleProduct loads a public product and its shop (catalog's rule:
// published and the shop active).
func (s *Service) visibleProduct(ctx context.Context, slug string) (catalog.Product, catalog.Shop, error) {
	p, err := orm.QueryOf[catalog.Product](s.db).Where("slug", slug).Where("status", catalog.StatusPublished).First(ctx)
	if errors.Is(err, orm.ErrNotFound) {
		return catalog.Product{}, catalog.Shop{}, ErrNotFound
	}

	if err != nil {
		return catalog.Product{}, catalog.Shop{}, fmt.Errorf("load product %q: %w", slug, err)
	}

	shop, err := orm.QueryOf[catalog.Shop](s.db).Where("id", p.ShopID).Where("status", catalog.ShopActive).First(ctx)
	if errors.Is(err, orm.ErrNotFound) {
		return catalog.Product{}, catalog.Shop{}, ErrNotFound
	}

	if err != nil {
		return catalog.Product{}, catalog.Shop{}, fmt.Errorf("load shop %d: %w", p.ShopID, err)
	}

	return p, shop, nil
}

// Ask stores askerID's question about the product at slug, then tells
// the shop owner (realtime notification and queued email). body must have
// passed AskSchema. The notifications are best effort: the question is
// saved even when they fail.
func (s *Service) Ask(ctx context.Context, askerID int64, slug, body string) (Question, error) {
	p, shop, err := s.visibleProduct(ctx, slug)
	if err != nil {
		return Question{}, err
	}

	if shop.OwnerID == askerID {
		return Question{}, ErrOwnProduct
	}

	q := Question{ProductID: p.ID, ShopID: shop.ID, AskerID: askerID, Body: body}
	if err := orm.Create(ctx, s.db, &q); err != nil {
		return Question{}, fmt.Errorf("store question on product %d: %w", p.ID, err)
	}

	s.tellOwner(ctx, shop.OwnerID, p, q)

	return q, nil
}

func (s *Service) tellOwner(ctx context.Context, ownerID int64, p catalog.Product, q Question) {
	if s.notify != nil {
		err := s.notify.Publish(ctx, ownerID, realtime.Notifications, map[string]any{
			"type": "qna.question", "question_id": q.ID, "product": p.Slug, "title": p.Title,
		})
		if err != nil {
			s.log.WarnContext(ctx, "qna: owner notification failed", "err", err, "question_id", q.ID)
		}
	}

	if s.mailOwner == nil {
		return
	}

	owner, err := orm.QueryOf[models.User](s.db).Find(ctx, ownerID)
	if err == nil {
		err = s.mailOwner(
			ctx,
			NewQuestionMail{Owner: owner, ProductTitle: p.Title, ProductSlug: p.Slug, Question: q.Body},
		)
	}

	if err != nil {
		s.log.WarnContext(ctx, "qna: owner email not queued", "err", err, "question_id", q.ID)
	}
}

// askerNames maps user ids to first names.
func (s *Service) askerNames(ctx context.Context, rows []Question) (map[int64]string, error) {
	ids := make([]any, 0, len(rows))
	for i := range rows {
		ids = append(ids, rows[i].AskerID)
	}

	out := map[int64]string{}
	if len(ids) == 0 {
		return out, nil
	}

	users, err := orm.QueryOf[models.User](s.db).Select("id", "name").WhereIn("id", ids...).Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("load askers: %w", err)
	}

	for i := range users {
		out[users[i].ID] = FirstName(users[i].Name)
	}

	return out, nil
}

func publicView(q Question, names map[int64]string) PublicQuestion {
	return PublicQuestion{
		ID: q.ID, Body: q.Body, Answer: q.Answer.V, AnsweredAt: timePtr(q.AnsweredAt),
		AskerName: names[q.AskerID], CreatedAt: q.CreatedAt,
	}
}

func newestFirst(q orm.Query[Question]) orm.Query[Question] {
	return q.OrderBy("created_at", orm.Desc).OrderBy("id", orm.Desc)
}

func pageOf[T any](res orm.Page[Question]) Page[T] {
	return Page[T]{
		Items: make([]T, 0, len(res.Items)), Total: res.Total, Page: res.Page,
		PerPage: res.PerPage, LastPage: res.LastPage,
	}
}

// ProductQuestions lists the visible questions of a product, newest first.
func (s *Service) ProductQuestions(ctx context.Context, productID int64, page int) (Page[PublicQuestion], error) {
	res, err := newestFirst(orm.QueryOf[Question](s.db).Where("product_id", productID).WhereNull("hidden_at")).
		Paginate(ctx, max(page, 1), PerPage)
	if err != nil {
		return Page[PublicQuestion]{}, fmt.Errorf("list questions of product %d: %w", productID, err)
	}

	names, err := s.askerNames(ctx, res.Items)
	if err != nil {
		return Page[PublicQuestion]{}, err
	}

	out := pageOf[PublicQuestion](res)
	for i := range res.Items {
		out.Items = append(out.Items, publicView(res.Items[i], names))
	}

	return out, nil
}

// Inbox filters of the seller questions page.
const (
	FilterUnanswered = "unanswered"
	FilterAnswered   = "answered"
)

// ShopQuestions lists the visible questions of ownerID's shop, newest
// first: unanswered ones, or answered ones with filter FilterAnswered.
func (s *Service) ShopQuestions(
	ctx context.Context,
	ownerID int64,
	filter string,
	page int,
) (Page[SellerQuestion], error) {
	shop, err := s.ownShop(ctx, ownerID)
	if err != nil {
		return Page[SellerQuestion]{}, err
	}

	q := orm.QueryOf[Question](s.db).Where("shop_id", shop.ID).WhereNull("hidden_at")
	if filter == FilterAnswered {
		q = q.WhereNotNull("answered_at")
	} else {
		q = q.WhereNull("answered_at")
	}

	res, err := newestFirst(q).Paginate(ctx, max(page, 1), PerPage)
	if err != nil {
		return Page[SellerQuestion]{}, fmt.Errorf("list questions of shop %d: %w", shop.ID, err)
	}

	names, err := s.askerNames(ctx, res.Items)
	if err != nil {
		return Page[SellerQuestion]{}, err
	}

	products, err := s.products(ctx, res.Items)
	if err != nil {
		return Page[SellerQuestion]{}, err
	}

	out := pageOf[SellerQuestion](res)
	for i := range res.Items {
		r := res.Items[i]
		out.Items = append(
			out.Items,
			SellerQuestion{PublicQuestion: publicView(r, names), Product: products[r.ProductID].ref},
		)
	}

	return out, nil
}

// UnansweredCount is the number of visible questions waiting for an
// answer in ownerID's shop.
func (s *Service) UnansweredCount(ctx context.Context, ownerID int64) (int64, error) {
	shop, err := s.ownShop(ctx, ownerID)
	if err != nil {
		return 0, err
	}

	n, err := orm.QueryOf[Question](s.db).Where("shop_id", shop.ID).WhereNull("hidden_at").WhereNull("answered_at").
		Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("count unanswered questions of shop %d: %w", shop.ID, err)
	}

	return n, nil
}

// OwnsShop reports whether userID owns shopID.
func (s *Service) OwnsShop(ctx context.Context, userID, shopID int64) (bool, error) {
	ok, err := orm.QueryOf[catalog.Shop](s.db).Where("id", shopID).Where("owner_id", userID).Exists(ctx)
	if err != nil {
		return false, fmt.Errorf("check owner of shop %d: %w", shopID, err)
	}

	return ok, nil
}

func (s *Service) ownShop(ctx context.Context, ownerID int64) (catalog.Shop, error) {
	shop, err := orm.QueryOf[catalog.Shop](s.db).Where("owner_id", ownerID).First(ctx)
	if errors.Is(err, orm.ErrNotFound) {
		return catalog.Shop{}, ErrNoShop
	}

	if err != nil {
		return catalog.Shop{}, fmt.Errorf("load shop of user %d: %w", ownerID, err)
	}

	return shop, nil
}

type productInfo struct {
	ref    ProductRef
	shopID int64
}

func (s *Service) products(ctx context.Context, rows []Question) (map[int64]productInfo, error) {
	ids := make([]any, 0, len(rows))
	for i := range rows {
		ids = append(ids, rows[i].ProductID)
	}

	out := map[int64]productInfo{}
	if len(ids) == 0 {
		return out, nil
	}

	ps, err := orm.QueryOf[catalog.Product](
		s.db,
	).Select("id", "shop_id", "slug", "title").
		WhereIn("id", ids...).
		Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("load question products: %w", err)
	}

	for i := range ps {
		out[ps[i].ID] = productInfo{ref: ProductRef{Slug: ps[i].Slug, Title: ps[i].Title}, shopID: ps[i].ShopID}
	}

	return out, nil
}

// Answer stores (or replaces) the answer of a visible question of
// ownerID's shop and tells the asker. answer must have passed
// AnswerSchema. Other shops' and hidden questions are ErrNotFound.
func (s *Service) Answer(ctx context.Context, ownerID, questionID int64, answer string) error {
	shop, err := s.ownShop(ctx, ownerID)
	if errors.Is(err, ErrNoShop) {
		return ErrNotFound
	}

	if err != nil {
		return err
	}

	q, err := orm.QueryOf[Question](s.db).Where("id", questionID).Where("shop_id", shop.ID).WhereNull("hidden_at").
		First(ctx)
	if errors.Is(err, orm.ErrNotFound) {
		return ErrNotFound
	}

	if err != nil {
		return fmt.Errorf("load question %d: %w", questionID, err)
	}

	if _, err := orm.QueryOf[Question](s.db).Where("id", q.ID).Update(ctx, orm.Set{
		"answer": answer, "answered_at": s.now().UTC(),
	}); err != nil {
		return fmt.Errorf("answer question %d: %w", q.ID, err)
	}

	if s.notify != nil {
		if err := s.notify.Publish(ctx, q.AskerID, realtime.Notifications, map[string]any{
			"type": "qna.answer", "question_id": q.ID,
		}); err != nil {
			s.log.WarnContext(ctx, "qna: asker notification failed", "err", err, "question_id", q.ID)
		}
	}

	return nil
}

// Admin filters of the moderation page.
const (
	FilterAll    = "all"
	FilterHidden = "hidden"
)

// AdminQuestions lists every question, newest first; FilterHidden keeps
// only hidden ones.
func (s *Service) AdminQuestions(ctx context.Context, filter string, page int) (Page[AdminQuestion], error) {
	q := orm.QueryOf[Question](s.db)
	if filter == FilterHidden {
		q = q.WhereNotNull("hidden_at")
	}

	res, err := newestFirst(q).Paginate(ctx, max(page, 1), PerPage*2)
	if err != nil {
		return Page[AdminQuestion]{}, fmt.Errorf("list questions: %w", err)
	}

	names, err := s.askerNames(ctx, res.Items)
	if err != nil {
		return Page[AdminQuestion]{}, err
	}

	products, err := s.products(ctx, res.Items)
	if err != nil {
		return Page[AdminQuestion]{}, err
	}

	shops, err := s.shopNames(ctx, res.Items)
	if err != nil {
		return Page[AdminQuestion]{}, err
	}

	out := pageOf[AdminQuestion](res)
	for i := range res.Items {
		r := res.Items[i]
		out.Items = append(out.Items, AdminQuestion{
			PublicQuestion: publicView(r, names), Product: products[r.ProductID].ref,
			AskerID: r.AskerID, ShopName: shops[r.ShopID],
			HiddenAt: timePtr(r.HiddenAt), HiddenReason: r.HiddenReason.V,
		})
	}

	return out, nil
}

func (s *Service) shopNames(ctx context.Context, rows []Question) (map[int64]string, error) {
	ids := make([]any, 0, len(rows))
	for i := range rows {
		ids = append(ids, rows[i].ShopID)
	}

	out := map[int64]string{}
	if len(ids) == 0 {
		return out, nil
	}

	shops, err := orm.QueryOf[catalog.Shop](s.db).Select("id", "name").WhereIn("id", ids...).Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("load question shops: %w", err)
	}

	for i := range shops {
		out[shops[i].ID] = shops[i].Name
	}

	return out, nil
}

// Hide takes a question off the public list and the seller inbox.
func (s *Service) Hide(ctx context.Context, adminID, questionID int64, reason string) error {
	return s.setHidden(ctx, questionID, orm.Set{
		"hidden_at": s.now().UTC(), "hidden_by": adminID, "hidden_reason": reason,
	})
}

// Unhide puts a hidden question back.
func (s *Service) Unhide(ctx context.Context, questionID int64) error {
	return s.setHidden(ctx, questionID, orm.Set{"hidden_at": nil, "hidden_by": nil, "hidden_reason": nil})
}

func (s *Service) setHidden(ctx context.Context, questionID int64, set orm.Set) error {
	n, err := orm.QueryOf[Question](s.db).Where("id", questionID).Update(ctx, set)
	if err != nil {
		return fmt.Errorf("moderate question %d: %w", questionID, err)
	}

	if n == 0 {
		return ErrNotFound
	}

	return nil
}
