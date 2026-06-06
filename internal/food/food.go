// Package food provides food search across an offline (bundled USDA) source and
// an online (USDA FoodData Central) source behind a common Provider interface.
// An offline-first composer (added later) queries offline instantly and appends
// online results, degrading silently to offline when the network is unavailable.
package food

import (
	"context"

	"caltui/internal/domain"
)

// Provider searches for foods matching a free-text query.
type Provider interface {
	// Search returns up to limit foods matching query. A blank query returns
	// no results.
	Search(ctx context.Context, query string, limit int) ([]domain.Food, error)
	// Name identifies the provider (e.g. "offline", "usda") for labeling.
	Name() string
}
