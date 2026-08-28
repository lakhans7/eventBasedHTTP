// Package marketplace defines the provider abstraction described in
// docs/architecture.md section 23. Nothing outside a specific provider
// package (e.g. internal/marketplace/fiverr) may call a marketplace's API
// directly — every caller goes through this interface, which makes adding
// a second marketplace (Upwork, Freelancer, ...) additive rather than
// invasive.
package marketplace

import (
	"context"
	"errors"

	"github.com/lakhans7/eventbasedhttp/internal/domain"
)

// ErrNotSupportedByProvider is returned by any capability a provider does not
// actually have. Callers must check Capabilities() before invoking a method,
// but every method returns this error too, defensively. See
// docs/fiverr-api-capabilities.md for why every Fiverr method returns it today.
var ErrNotSupportedByProvider = errors.New("not supported by this marketplace provider")

// Capabilities describes what a provider can actually do, so the API and UI
// layers can hide/disable actions instead of exposing a button that always
// fails.
type Capabilities struct {
	AccountRead bool
	GigRead     bool
	GigWrite    bool
	OrderRead   bool
	OrderWrite  bool
	MessageRead bool
	MessageSend bool
	ReviewRead  bool
	Webhooks    bool
}

// Provider is implemented once per marketplace. FiverrProvider is the only
// implementation today; see docs/architecture.md "Multi-marketplace
// extensibility" for how a second one plugs in.
type Provider interface {
	Name() string
	Capabilities() Capabilities

	GetAccount(ctx context.Context, accountID string) (*domain.FiverrAccount, error)
	GetGigs(ctx context.Context, accountID string) ([]domain.Gig, error)
	GetOrders(ctx context.Context, accountID string) ([]domain.Order, error)
	GetConversations(ctx context.Context, accountID string) ([]domain.Conversation, error)
	GetMessages(ctx context.Context, conversationID string) ([]domain.Message, error)
	GetReviews(ctx context.Context, accountID string) ([]domain.Review, error)
}
