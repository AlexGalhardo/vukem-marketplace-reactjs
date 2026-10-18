package reviews

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/AlexGalhardo/vukem/database"

	"github.com/AlexGalhardo/vukem-marketplace-reactjs/app/dbx"
)

// Summary is a product's rating: the stored average and count (the same
// numbers the product cards show) and how many visible reviews gave each
// star, Distribution[0] being 1 star.
type Summary struct {
	AvgX100      int    `json:"avg_x100"`
	Count        int    `json:"count"`
	Distribution [5]int `json:"distribution"`
}

// Summary returns the rating summary of productID.
func (s *Service) Summary(ctx context.Context, productID int64) (Summary, error) {
	var out Summary

	if err := dbx.QueryRow(ctx, s.DB, `SELECT rating_avg_x100, rating_count FROM products WHERE id = ?`, productID).
		Scan(&out.AvgX100, &out.Count); err != nil {
		return Summary{}, fmt.Errorf("load rating of product %d: %w", productID, err)
	}

	rows, err := s.query(ctx, `SELECT rating, COUNT(*) FROM reviews
		WHERE product_id = ? AND hidden_at IS NULL GROUP BY rating`, productID)
	if err != nil {
		return Summary{}, fmt.Errorf("rating distribution of product %d: %w", productID, err)
	}
	defer rows.Close()

	for rows.Next() {
		var stars, n int
		if err := rows.Scan(&stars, &n); err != nil {
			return Summary{}, err
		}

		if stars >= 1 && stars <= 5 {
			out.Distribution[stars-1] = n
		}
	}

	return out, rows.Err()
}

// Reply is the shop's answer to a review.
type Reply struct {
	Body string    `json:"body"`
	At   time.Time `json:"at"`
}

// Review is a review as the product page shows it.
type Review struct {
	ID     int64  `json:"id"`
	Rating int    `json:"rating"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	// Recommend is nil when the buyer did not answer.
	Recommend    *bool     `json:"recommend"`
	HelpfulCount int       `json:"helpful_count"`
	Author       string    `json:"author"`
	CreatedAt    time.Time `json:"created_at"`
	Reply        *Reply    `json:"reply"`
	// Verified: every review comes from a paid order line.
	Verified bool `json:"verified"`
	// Own and Voted describe the viewer: their own review, or one they
	// already found helpful.
	Own   bool `json:"own"`
	Voted bool `json:"voted"`
}

// Page is one page of a product's reviews.
type Page struct {
	Data     []Review `json:"data"`
	Total    int      `json:"total"`
	Page     int      `json:"page"`
	PerPage  int      `json:"per_page"`
	LastPage int      `json:"last_page"`
	Sort     string   `json:"sort"`
}

// firstName keeps the first word of a name: reviews show "Ana", not the
// buyer's full name.
func firstName(name string) string {
	if f := strings.Fields(name); len(f) > 0 {
		return f[0]
	}

	return ""
}

func lastPage(total, perPage int) int { return max(1, (total+perPage-1)/perPage) }

// List returns page of productID's visible reviews in sort order (newest
// or most helpful first), as viewerID (0 for guests) sees them.
func (s *Service) List(ctx context.Context, productID, viewerID int64, sort string, page int) (Page, error) {
	if sort != SortHelpful {
		sort = SortNewest
	}

	out := Page{Data: []Review{}, Page: max(page, 1), PerPage: PerPage, Sort: sort}

	if err := dbx.QueryRow(ctx, s.DB, `SELECT COUNT(*) FROM reviews WHERE product_id = ? AND hidden_at IS NULL`,
		productID).Scan(&out.Total); err != nil {
		return Page{}, fmt.Errorf("count reviews of product %d: %w", productID, err)
	}

	out.LastPage = lastPage(out.Total, PerPage)
	out.Page = min(out.Page, out.LastPage)

	order := `r.created_at DESC, r.id DESC`
	if sort == SortHelpful {
		order = `r.helpful_count DESC, r.created_at DESC, r.id DESC`
	}

	// order is one of two constants above; the rest is bound.
	rows, err := s.query(ctx, (`SELECT r.id, r.rating, COALESCE(r.title, ''),
		COALESCE(r.body, ''), r.recommend, r.helpful_count, u.name, r.created_at, r.seller_reply, r.seller_replied_at,
		r.buyer_id, EXISTS (SELECT 1 FROM review_votes v WHERE v.review_id = r.id AND v.user_id = ?)
		FROM reviews r JOIN users u ON u.id = r.buyer_id
		WHERE r.product_id = ? AND r.hidden_at IS NULL
		ORDER BY ` + order + ` LIMIT ? OFFSET ?`), viewerID, productID, PerPage, (out.Page-1)*PerPage)
	if err != nil {
		return Page{}, fmt.Errorf("list reviews of product %d: %w", productID, err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			r         Review
			recommend sql.Null[bool]
			reply     sql.Null[string]
			repliedAt sql.Null[time.Time]
			buyerID   int64
		)

		if err := rows.Scan(&r.ID, &r.Rating, &r.Title, &r.Body, &recommend, &r.HelpfulCount, &r.Author,
			&r.CreatedAt, &reply, &repliedAt, &buyerID, &r.Voted); err != nil {
			return Page{}, fmt.Errorf("scan review: %w", err)
		}

		r.Author = firstName(r.Author)
		r.Verified = true
		r.Own = viewerID != 0 && buyerID == viewerID

		if recommend.Valid {
			r.Recommend = &recommend.V
		}

		if reply.Valid {
			r.Reply = &Reply{Body: reply.V, At: repliedAt.V}
		}

		out.Data = append(out.Data, r)
	}

	return out, rows.Err()
}

// AdminReview is a review in the moderation list.
type AdminReview struct {
	ID           int64      `json:"id"`
	Rating       int        `json:"rating"`
	Title        string     `json:"title"`
	Body         string     `json:"body"`
	Author       string     `json:"author"`
	ProductSlug  string     `json:"product_slug"`
	ProductTitle string     `json:"product_title"`
	CreatedAt    time.Time  `json:"created_at"`
	HiddenAt     *time.Time `json:"hidden_at"`
	HiddenReason string     `json:"hidden_reason"`
}

// AdminPage is one page of the moderation list.
type AdminPage struct {
	Data     []AdminReview `json:"data"`
	Total    int           `json:"total"`
	Page     int           `json:"page"`
	PerPage  int           `json:"per_page"`
	LastPage int           `json:"last_page"`
	Filter   string        `json:"filter"`
}

// Moderation filters.
const (
	FilterAll    = "all"
	FilterHidden = "hidden"
)

// AdminList returns the newest reviews of every product, hidden ones
// included (only hidden ones with FilterHidden).
func (s *Service) AdminList(ctx context.Context, filter string, page int) (AdminPage, error) {
	where := `1 = 1`
	if filter == FilterHidden {
		where = `r.hidden_at IS NOT NULL`
	} else {
		filter = FilterAll
	}

	out := AdminPage{Data: []AdminReview{}, Page: max(page, 1), PerPage: AdminPerPage, Filter: filter}

	// where is one of two constants above.
	if err := dbx.QueryRow(ctx, s.DB, `SELECT COUNT(*) FROM reviews r WHERE `+where).Scan(&out.Total); err != nil {
		return AdminPage{}, fmt.Errorf("count reviews: %w", err)
	}

	out.LastPage = lastPage(out.Total, AdminPerPage)
	out.Page = min(out.Page, out.LastPage)

	rows, err := s.query(ctx, (`SELECT r.id, r.rating, COALESCE(r.title, ''),
		COALESCE(r.body, ''), u.name, p.slug, p.title, r.created_at, r.hidden_at, COALESCE(r.hidden_reason, '')
		FROM reviews r JOIN users u ON u.id = r.buyer_id JOIN products p ON p.id = r.product_id
		WHERE ` + where + ` ORDER BY r.created_at DESC, r.id DESC LIMIT ? OFFSET ?`),
		AdminPerPage, (out.Page-1)*AdminPerPage)
	if err != nil {
		return AdminPage{}, fmt.Errorf("list reviews: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			r      AdminReview
			hidden sql.Null[time.Time]
		)

		if err := rows.Scan(&r.ID, &r.Rating, &r.Title, &r.Body, &r.Author, &r.ProductSlug, &r.ProductTitle,
			&r.CreatedAt, &hidden, &r.HiddenReason); err != nil {
			return AdminPage{}, fmt.Errorf("scan review: %w", err)
		}

		if hidden.Valid {
			r.HiddenAt = &hidden.V
		}

		out.Data = append(out.Data, r)
	}

	return out, rows.Err()
}

// query runs a "?" query built from constant pieces; RebindChecked fails
// closed on an unbalanced literal.
func (s *Service) query(ctx context.Context, q string, args ...any) (*sql.Rows, error) {
	bound, err := database.RebindChecked(s.DB.Dialect(), q)
	if err != nil {
		return nil, err
	}

	return s.DB.QueryContext(ctx, bound, args...)
}

// OwnsShop reports whether userID owns shop shopID (the shop may reply to
// the reviews of its products).
func (s *Service) OwnsShop(ctx context.Context, userID, shopID int64) (bool, error) {
	var n int
	if err := dbx.QueryRow(ctx, s.DB, `SELECT COUNT(*) FROM shops WHERE id = ? AND owner_id = ?`, shopID, userID).
		Scan(&n); err != nil {
		return false, fmt.Errorf("load shop %d: %w", shopID, err)
	}

	return n == 1, nil
}
