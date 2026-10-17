package catalog

import (
	"context"
	"fmt"
	"time"

	"github.com/AlexGalhardo/vukem/cache"
	"github.com/AlexGalhardo/vukem/orm"
)

// CategoryView is a category with both names (pages pick one by locale).
type CategoryView struct {
	Slug   string `json:"slug"`
	NameEN string `json:"name_en"`
	NamePT string `json:"name_pt"`
	Parent string `json:"parent"`
}

// Name returns the name in locale.
func (c CategoryView) Name(locale string) string {
	if locale == "pt-BR" {
		return c.NamePT
	}

	return c.NameEN
}

// Ref returns the category's slug and localized name.
func (c CategoryView) Ref(locale string) CategoryRef {
	return CategoryRef{Slug: c.Slug, Name: c.Name(locale)}
}

const categoriesTTL = time.Minute

// Categories lists every category in display order (cached for a minute:
// categories only change through seeders and admins).
func (s *Service) Categories(ctx context.Context) ([]CategoryView, error) {
	load := func(ctx context.Context) ([]CategoryView, error) {
		rows, err := orm.QueryOf[Category](s.db).OrderBy("position", orm.Asc).OrderBy("id", orm.Asc).Get(ctx)
		if err != nil {
			return nil, fmt.Errorf("list categories: %w", err)
		}

		slugs := make(map[int64]string, len(rows))
		for i := range rows {
			slugs[rows[i].ID] = rows[i].Slug
		}

		out := make([]CategoryView, 0, len(rows))
		for i := range rows {
			r := &rows[i]
			out = append(
				out,
				CategoryView{Slug: r.Slug, NameEN: r.NameEN, NamePT: r.NamePT, Parent: slugs[r.ParentID.V]},
			)
		}

		return out, nil
	}

	if s.cache == nil {
		return load(ctx)
	}

	return cache.Remember(ctx, s.cache, "catalog:categories", categoriesTTL, load)
}

// CategoryBySlug returns one category, or ErrNotFound.
func (s *Service) CategoryBySlug(ctx context.Context, slug string) (CategoryView, error) {
	all, err := s.Categories(ctx)
	if err != nil {
		return CategoryView{}, err
	}

	for _, c := range all {
		if c.Slug == slug {
			return c, nil
		}
	}

	return CategoryView{}, ErrNotFound
}

// LocalizedCategories returns slug + name pairs in locale.
func LocalizedCategories(all []CategoryView, locale string) []CategoryRef {
	out := make([]CategoryRef, 0, len(all))
	for _, c := range all {
		out = append(out, c.Ref(locale))
	}

	return out
}
