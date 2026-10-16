package postgres

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/wecratfs/commerce/internal/domain/storefront"
	"net/url"
	"strings"
)

func validateStorefrontReferences(ctx context.Context, tx pgx.Tx, c storefront.Config) error {
	categories := map[string]bool{}
	for _, v := range c.Categories {
		categories[v.Slug] = true
	}
	sections := append([]storefront.Section{}, c.Homepage...)
	for _, p := range c.Pages {
		sections = append(sections, p.Sections...)
	}
	for _, t := range c.Templates {
		sections = append(sections, t.Sections...)
		if strings.HasPrefix(t.Match, "category:") && !categories[strings.TrimPrefix(t.Match, "category:")] {
			return fmt.Errorf("%w: template category was removed", storefront.ErrValidation)
		}
		if strings.HasPrefix(t.Match, "product:") {
			var exists bool
			if e := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM products WHERE slug=$1)`, strings.TrimPrefix(t.Match, "product:")).Scan(&exists); e != nil {
				return e
			}
			if !exists {
				return fmt.Errorf("%w: template product was removed", storefront.ErrValidation)
			}
		}
	}
	for _, s := range sections {
		if s.Selection.Category != "" && !categories[s.Selection.Category] {
			return fmt.Errorf("%w: section %s refers to a removed category", storefront.ErrValidation, s.Title)
		}
		if s.Type == "products" && s.Selection.Mode == "selected" {
			var n int
			if e := tx.QueryRow(ctx, `SELECT count(*) FROM products WHERE id=ANY($1::bigint[])`, s.Selection.Products).Scan(&n); e != nil {
				return e
			}
			if n != len(s.Selection.Products) {
				return fmt.Errorf("%w: a selected product in %s was removed; update the selection", storefront.ErrValidation, s.Title)
			}
		}
	}
	images := []string{c.Theme.Logo}
	for _, slide := range c.Slides {
		images = append(images, slide.Image, slide.MobileImage)
	}
	for _, cat := range c.Categories {
		images = append(images, cat.Image)
	}
	for _, s := range sections {
		images = append(images, s.Image, s.MobileImage)
		for _, tile := range s.Items {
			images = append(images, tile.Image)
		}
	}
	for _, tile := range c.Trust {
		images = append(images, tile.Image)
	}
	for _, src := range images {
		if !strings.HasPrefix(src, "/media/storefront/") {
			continue
		}
		parsed, e := url.Parse(src)
		if e != nil {
			return storefront.ErrValidation
		}
		id := strings.TrimPrefix(parsed.Path, "/media/storefront/")
		var exists bool
		if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM storefront_media WHERE id=$1)`, id).Scan(&exists); e != nil {
			return e
		}
		if !exists {
			return fmt.Errorf("%w: an uploaded photograph is missing; choose another from Media", storefront.ErrValidation)
		}
	}
	return nil
}
