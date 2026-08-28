// Package fiverr is the only MarketplaceProvider implementation today.
// Per docs/fiverr-api-capabilities.md, Fiverr publishes no OAuth/API for
// third-party seller-account access, so every Provider method below
// honestly returns marketplace.ErrNotSupportedByProvider instead of faking
// a connection. The real, compliant functionality this package offers is
// CSV import and manual data entry — see importer.go and handlers.go.
package fiverr

import (
	"context"

	"github.com/lakhans7/eventbasedhttp/internal/domain"
	"github.com/lakhans7/eventbasedhttp/internal/marketplace"
)

type Provider struct{}

func NewProvider() *Provider {
	return &Provider{}
}

func (p *Provider) Name() string { return "fiverr" }

func (p *Provider) Capabilities() marketplace.Capabilities {
	// Every flag is false: there is no live Fiverr API to call. This is
	// intentionally not a placeholder to "fill in later" without
	// re-verifying docs/fiverr-api-capabilities.md first.
	return marketplace.Capabilities{}
}

func (p *Provider) GetAccount(ctx context.Context, accountID string) (*domain.FiverrAccount, error) {
	return nil, marketplace.ErrNotSupportedByProvider
}

func (p *Provider) GetGigs(ctx context.Context, accountID string) ([]domain.Gig, error) {
	return nil, marketplace.ErrNotSupportedByProvider
}

func (p *Provider) GetOrders(ctx context.Context, accountID string) ([]domain.Order, error) {
	return nil, marketplace.ErrNotSupportedByProvider
}

func (p *Provider) GetConversations(ctx context.Context, accountID string) ([]domain.Conversation, error) {
	return nil, marketplace.ErrNotSupportedByProvider
}

func (p *Provider) GetMessages(ctx context.Context, conversationID string) ([]domain.Message, error) {
	return nil, marketplace.ErrNotSupportedByProvider
}

func (p *Provider) GetReviews(ctx context.Context, accountID string) ([]domain.Review, error) {
	return nil, marketplace.ErrNotSupportedByProvider
}
