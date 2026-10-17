// Package reviews is the product review service: eligibility, writing a
// review, the product summary and list, "helpful" votes, the shop's reply
// and admin moderation. Rules that live here and nowhere else:
//
//   - only the buyer of an order line may review it, once (order_item_id is
//     unique), while its seller order is paid, shipped or delivered
//     (EligibleStatuses); a line of the reviewer's own shop never is;
//   - anything the acting user may not see or act on is ErrNotFound, so ids
//     do not leak;
//   - products.rating_avg_x100 and rating_count cover the visible (not
//     hidden) reviews and change in the same transaction as the review,
//     with the product row locked; RecalculateRatings rebuilds them.
package reviews

import (
	"errors"
	"time"

	"github.com/AlexGalhardo/vukem/database"
)

// Field limits shared by the schemas and the pages.
const (
	MaxTitle     = 80
	MaxBody      = 2000
	MaxReply     = 1000
	MinReason    = 3
	MaxReason    = 500
	PerPage      = 10
	AdminPerPage = 20
)

// EligibleStatuses are the seller order statuses whose lines can be
// reviewed: the buyer paid and was not refunded or canceled. Requiring
// "delivered" would lock reviews out until the shipping flow (PLAN O.1)
// marks orders delivered.
var EligibleStatuses = []string{"paid", "shipped", "delivered"}

// Errors. Controllers map them to 404 or to a form error.
var (
	// ErrNotFound: no such order line, review or product for this user, or
	// not eligible (pending, refunded, someone else's, own shop).
	ErrNotFound = errors.New("reviews: not found")
	// ErrAlreadyReviewed: the order line has its review.
	ErrAlreadyReviewed = errors.New("reviews: order line already reviewed")
	// ErrAlreadyReplied: the shop already answered this review.
	ErrAlreadyReplied = errors.New("reviews: review already has a reply")
	// ErrOwnReview: authors cannot vote their own review helpful.
	ErrOwnReview = errors.New("reviews: cannot vote own review")
)

// Sort orders of the product list.
const (
	SortNewest  = "newest"
	SortHelpful = "helpful"
)

// Service reads and writes reviews.
type Service struct {
	DB  *database.DB
	Now func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now == nil {
		return time.Now().UTC()
	}

	return s.Now().UTC()
}

// statusArgs returns EligibleStatuses as query arguments.
func statusArgs() []any {
	out := make([]any, len(EligibleStatuses))
	for i, st := range EligibleStatuses {
		out[i] = st
	}

	return out
}
