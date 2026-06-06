// Package offline implements food.Provider over the bundled food database using
// SQLite FTS5 prefix search. It is always available and returns instantly.
package offline

import (
	"context"

	"caltui/internal/domain"
	"caltui/internal/store"
)

// Provider searches the local foods table.
type Provider struct {
	store *store.Store
}

// New returns an offline provider backed by s.
func New(s *store.Store) *Provider { return &Provider{store: s} }

// Name identifies this provider.
func (p *Provider) Name() string { return "offline" }

// Search runs a full-text prefix search over the bundled foods. The context is
// accepted for interface symmetry; the query is local and effectively instant.
func (p *Provider) Search(_ context.Context, query string, limit int) ([]domain.Food, error) {
	return p.store.SearchFoods(query, limit)
}
