// Package domain contains the marketplace-agnostic internal models. Every
// MarketplaceProvider (Fiverr today, others later) normalizes into these
// types; nothing outside internal/marketplace/<provider> knows about a
// specific marketplace's data shape. See docs/architecture.md.
package domain

import "time"

type User struct {
	ID              string     `json:"id"`
	Email           string     `json:"email"`
	Name            string     `json:"name"`
	PasswordHash    string     `json:"-"`
	AvatarURL       string     `json:"avatar_url,omitempty"`
	Status          string     `json:"status"`
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"`
	LastLoginAt     *time.Time `json:"last_login_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

const (
	UserStatusActive          = "active"
	UserStatusSuspended       = "suspended"
	UserStatusPendingDeletion = "pending_deletion"
)

// ConnectionMethod describes how a FiverrAccount's data got into our system.
// "manual" is the only supported value today because Fiverr has no public
// OAuth/API (see docs/fiverr-api-capabilities.md). "oauth" is reserved for
// if/when Fiverr ever ships one.
type ConnectionMethod string

const (
	ConnectionMethodManual ConnectionMethod = "manual"
	ConnectionMethodOAuth  ConnectionMethod = "oauth"
)

const (
	FiverrAccountStatusConnected    = "connected"
	FiverrAccountStatusDisconnected = "disconnected"
)

type FiverrAccount struct {
	ID                string           `json:"id"`
	UserID            string           `json:"user_id"`
	ExternalAccountID *string          `json:"external_account_id,omitempty"`
	Username          string           `json:"username"`
	ConnectionMethod  ConnectionMethod `json:"connection_method"`
	Status            string           `json:"status"`
	Scopes            []string         `json:"scopes,omitempty"`
	TokenExpiresAt    *time.Time       `json:"token_expires_at,omitempty"`
	ConnectedAt       time.Time        `json:"connected_at"`
	LastSyncAt        *time.Time       `json:"last_sync_at,omitempty"`
	CreatedAt         time.Time        `json:"created_at"`
	UpdatedAt         time.Time        `json:"updated_at"`
}

type Customer struct {
	ID              string    `json:"id"`
	FiverrAccountID string    `json:"fiverr_account_id"`
	ExternalRef     string    `json:"external_ref"`
	DisplayName     string    `json:"display_name"`
	Notes           string    `json:"notes,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

type GigPackage struct {
	Name         string `json:"name"`
	PriceCents   int64  `json:"price_cents"`
	DeliveryDays int    `json:"delivery_days"`
	Description  string `json:"description,omitempty"`
}

type GigMetrics struct {
	Views  *int      `json:"views,omitempty"`
	Orders *int      `json:"orders,omitempty"`
	Rating *float64  `json:"rating,omitempty"`
	AsOf   time.Time `json:"as_of"`
}

const (
	GigSourceManual = "manual"
	GigSourceImport = "csv_import"

	GigStatusActive  = "active"
	GigStatusPaused  = "paused"
	GigStatusDenied  = "denied"
	GigStatusUnknown = "unknown"
)

// Gig fields below mirror the sections of Fiverr's own gig-creation wizard
// (Overview / Pricing / Description & FAQ / Requirements) so the "copy to
// Fiverr" flow (web/gig-copy.html) can present them in the same order a
// seller fills them into Fiverr's real gig editor. Nothing here is ever sent
// to Fiverr — there is no gig-write API (docs/fiverr-api-capabilities.md).
type Gig struct {
	ID                string       `json:"id"`
	FiverrAccountID   string       `json:"fiverr_account_id"`
	ExternalRef       *string      `json:"external_ref,omitempty"`
	Title             string       `json:"title"`
	Category          string       `json:"category,omitempty"`
	SubCategory       string       `json:"sub_category,omitempty"`
	Tags              []string     `json:"tags,omitempty"`
	Status            string       `json:"status"`
	BasePriceCents    int64        `json:"base_price_cents"`
	Currency          string       `json:"currency"`
	Packages          []GigPackage `json:"packages,omitempty"`
	Description       string       `json:"description,omitempty"`
	FAQs              []FAQ        `json:"faqs,omitempty"`
	BuyerRequirements string       `json:"buyer_requirements,omitempty"`
	Metrics           *GigMetrics  `json:"metrics,omitempty"`
	Source            string       `json:"source"`
	CreatedAt         time.Time    `json:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at"`
}

const (
	OrderStatusActive            = "active"
	OrderStatusDelivered         = "delivered"
	OrderStatusRevisionRequested = "revision_requested"
	OrderStatusCompleted         = "completed"
	OrderStatusCancelled         = "cancelled"
	OrderStatusLate              = "late"

	OrderStageCreated              = "created"
	OrderStageRequirementsReceived = "requirements_received"
	OrderStageInProgress           = "in_progress"
	OrderStageDelivered            = "delivered"
	OrderStageRevisionRequested    = "revision_requested"
	OrderStageRevisionSubmitted    = "revision_submitted"
	OrderStageCompleted            = "completed"

	OrderSourceManual = "manual"
	OrderSourceImport = "csv_import"
)

type Order struct {
	ID              string     `json:"id"`
	FiverrAccountID string     `json:"fiverr_account_id"`
	GigID           *string    `json:"gig_id,omitempty"`
	CustomerID      string     `json:"customer_id"`
	ExternalRef     *string    `json:"external_ref,omitempty"`
	AmountCents     int64      `json:"amount_cents"`
	Currency        string     `json:"currency"`
	Status          string     `json:"status"`
	Stage           string     `json:"stage"`
	DueAt           *time.Time `json:"due_at,omitempty"`
	DeliveredAt     *time.Time `json:"delivered_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	CancelledAt     *time.Time `json:"cancelled_at,omitempty"`
	Source          string     `json:"source"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type RequirementExtraction struct {
	Technologies []string `json:"technologies"`
	Features     []string `json:"features"`
	Missing      []string `json:"missing"`
}

type OrderRequirement struct {
	ID        string                 `json:"id"`
	OrderID   string                 `json:"order_id"`
	RawText   string                 `json:"raw_text"`
	Extracted *RequirementExtraction `json:"extracted,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

type Conversation struct {
	ID              string    `json:"id"`
	FiverrAccountID string    `json:"fiverr_account_id"`
	CustomerID      string    `json:"customer_id"`
	GigID           *string   `json:"gig_id,omitempty"`
	LastMessageAt   time.Time `json:"last_message_at"`
	UnreadCount     int       `json:"unread_count"`
	CreatedAt       time.Time `json:"created_at"`
}

const (
	MessageDirectionInbound  = "inbound"
	MessageDirectionOutbound = "outbound"

	MessageSourceManualPaste = "manual_paste"
	MessageSourceImport      = "csv_import"
)

type Message struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	Direction      string    `json:"direction"`
	Body           string    `json:"body"`
	SentAt         time.Time `json:"sent_at"`
	Source         string    `json:"source"`
	AIGenerationID *string   `json:"ai_generation_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type Review struct {
	ID              string    `json:"id"`
	FiverrAccountID string    `json:"fiverr_account_id"`
	GigID           *string   `json:"gig_id,omitempty"`
	OrderID         *string   `json:"order_id,omitempty"`
	Rating          int       `json:"rating"`
	Body            string    `json:"body,omitempty"`
	Sentiment       *string   `json:"sentiment,omitempty"`
	PostedAt        time.Time `json:"posted_at"`
	CreatedAt       time.Time `json:"created_at"`
}

const (
	NotificationTypeNewMessage        = "new_message"
	NotificationTypeNewOrder          = "new_order"
	NotificationTypeDeadline          = "deadline"
	NotificationTypeRevisionRequested = "revision_requested"
	NotificationTypeDelivery          = "delivery"
	NotificationTypeSyncFailure       = "sync_failure"
	NotificationTypeConnectionFailure = "connection_failure"
)

type Notification struct {
	ID           string     `json:"id"`
	UserID       string     `json:"user_id"`
	Type         string     `json:"type"`
	Title        string     `json:"title"`
	Body         string     `json:"body"`
	ResourceType *string    `json:"resource_type,omitempty"`
	ResourceID   *string    `json:"resource_id,omitempty"`
	ReadAt       *time.Time `json:"read_at,omitempty"`
	Channels     []string   `json:"channels"`
	CreatedAt    time.Time  `json:"created_at"`
}

const (
	AIGenerationStatusPendingReview  = "pending_review"
	AIGenerationStatusEdited         = "edited"
	AIGenerationStatusApproved       = "approved"
	AIGenerationStatusRejected       = "rejected"
	AIGenerationStatusSentExternally = "sent_externally"

	AIKindMessageReply       = "message_reply"
	AIKindRequirementSummary = "requirement_summary"
	AIKindDeliveryMessage    = "delivery_message"
	AIKindReviewAnalysis     = "review_analysis"
	AIKindInsight            = "insight"
)

type AIGeneration struct {
	ID               string         `json:"id"`
	UserID           string         `json:"user_id"`
	Kind             string         `json:"kind"`
	ContextRef       map[string]any `json:"context_ref,omitempty"`
	Model            string         `json:"model"`
	InputTokens      int            `json:"input_tokens"`
	OutputTokens     int            `json:"output_tokens"`
	EstimatedCostUSD float64        `json:"estimated_cost_usd"`
	PromptRedacted   string         `json:"-"`
	DraftOutput      string         `json:"draft_output"`
	RiskFlags        []string       `json:"risk_flags,omitempty"`
	Status           string         `json:"status"`
	ApprovedBy       *string        `json:"approved_by,omitempty"`
	ApprovedAt       *time.Time     `json:"approved_at,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

type SellerPreferences struct {
	UserID                 string   `json:"user_id"`
	Skills                 []string `json:"skills"`
	Services               []string `json:"services"`
	Tone                   string   `json:"tone"`
	MinProjectUSD          int      `json:"min_project_usd"`
	TypicalDeliveryDaysMin int      `json:"typical_delivery_days_min"`
	TypicalDeliveryDaysMax int      `json:"typical_delivery_days_max"`
	FAQs                   []FAQ    `json:"faqs"`
	PortfolioLinks         []string `json:"portfolio_links"`
	Terms                  string   `json:"terms,omitempty"`
	Restrictions           string   `json:"restrictions,omitempty"`
}

type FAQ struct {
	Question string `json:"q"`
	Answer   string `json:"a"`
}

const (
	SyncJobStatusQueued    = "queued"
	SyncJobStatusRunning   = "running"
	SyncJobStatusSucceeded = "succeeded"
	SyncJobStatusFailed    = "failed"
)

type SyncJob struct {
	ID              string     `json:"id"`
	FiverrAccountID *string    `json:"fiverr_account_id,omitempty"`
	JobType         string     `json:"job_type"`
	Status          string     `json:"status"`
	Attempt         int        `json:"attempt"`
	Error           *string    `json:"error,omitempty"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

type AuditLog struct {
	ID           string         `json:"id"`
	UserID       *string        `json:"user_id,omitempty"`
	Action       string         `json:"action"`
	ResourceType *string        `json:"resource_type,omitempty"`
	ResourceID   *string        `json:"resource_id,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	IPAddress    string         `json:"ip_address,omitempty"`
	UserAgent    string         `json:"user_agent,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}
