package catalog

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Suggest limits: the navbar search shows the top MaxSuggestions products
// once the term reaches MinSuggestChars characters.
const (
	MinSuggestChars = 3
	MaxSuggestions  = 8
)

// Suggestion is one navbar search result.
type Suggestion struct {
	Slug       string `json:"slug"`
	Title      string `json:"title"`
	Category   string `json:"category"`
	ImageURL   string `json:"image_url"`
	PriceCents int64  `json:"price_cents"`
}

// Suggest returns the best MaxSuggestions visible products for term, with
// the category name in locale. Terms shorter than MinSuggestChars return
// nothing.
func (s *Service) Suggest(ctx context.Context, term, locale string, cached bool) ([]Suggestion, error) {
	term = strings.Join(strings.Fields(term), " ")
	if utf8.RuneCountInString(term) < MinSuggestChars || utf8.RuneCountInString(term) > MaxSearchTermLen {
		return []Suggestion{}, nil
	}

	page, err := s.Search(ctx, Query{Q: term, PerPage: MaxSuggestions}, cached)
	if err != nil {
		return nil, err
	}

	if len(page.Items) == 0 {
		return []Suggestion{}, nil
	}

	ids := make([]int64, len(page.Items))
	for i := range page.Items {
		ids[i] = page.Items[i].ID
	}

	cats, err := s.Categories(ctx)
	if err != nil {
		return nil, err
	}

	names := make(map[string]string, len(cats))
	for _, c := range cats {
		names[c.Slug] = c.Name(locale)
	}

	b := newSQL(s.db.Dialect())

	rows, err := s.db.QueryContext(ctx, `SELECT p.id, c.slug FROM products p JOIN categories c ON c.id = p.category_id
		WHERE p.id IN (`+b.list(ids)+`)`, b.args...)
	if err != nil {
		return nil, fmt.Errorf("suggest categories: %w", err)
	}
	defer rows.Close()

	slugOf := make(map[int64]string, len(ids))

	for rows.Next() {
		var (
			id   int64
			slug string
		)

		if err := rows.Scan(&id, &slug); err != nil {
			return nil, fmt.Errorf("scan suggest category: %w", err)
		}

		slugOf[id] = slug
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("suggest categories: %w", err)
	}

	out := make([]Suggestion, len(page.Items))
	for i := range page.Items {
		c := &page.Items[i]
		out[i] = Suggestion{
			Slug: c.Slug, Title: c.Title, Category: names[slugOf[c.ID]], ImageURL: c.ImageURL, PriceCents: c.PriceCents,
		}
	}

	return out, nil
}
