package store

import (
	"context"
	"testing"

	"github.com/lakhans7/eventbasedhttp/internal/domain"
)

func TestFiverrAccountAndGigRoundTrip(t *testing.T) {
	st := setupTestPool(t)
	ctx := context.Background()
	userID := createTestUser(t, st)

	account, err := st.CreateFiverrAccount(ctx, userID, "test_seller")
	if err != nil {
		t.Fatalf("CreateFiverrAccount: %v", err)
	}
	if account.ConnectionMethod != domain.ConnectionMethodManual {
		t.Fatalf("expected manual connection method, got %s", account.ConnectionMethod)
	}

	gig, err := st.UpsertGig(ctx, GigInput{
		FiverrAccountID: account.ID,
		Title:           "I will build a Go backend",
		BasePriceCents:  10000,
		Source:          domain.GigSourceManual,
	})
	if err != nil {
		t.Fatalf("UpsertGig: %v", err)
	}

	fetched, err := st.GetGig(ctx, gig.ID, userID)
	if err != nil {
		t.Fatalf("GetGig: %v", err)
	}
	if fetched.Title != "I will build a Go backend" || fetched.BasePriceCents != 10000 {
		t.Fatalf("unexpected gig: %+v", fetched)
	}

	if err := st.DisconnectFiverrAccount(ctx, account.ID, userID); err != nil {
		t.Fatalf("DisconnectFiverrAccount: %v", err)
	}
	if _, err := st.GetFiverrAccount(ctx, account.ID, userID); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound after disconnect, got %v", err)
	}
}

func TestOrderOwnershipIsEnforced(t *testing.T) {
	st := setupTestPool(t)
	ctx := context.Background()
	ownerID := createTestUser(t, st)
	strangerID := createTestUser(t, st)

	account, err := st.CreateFiverrAccount(ctx, ownerID, "owner_account")
	if err != nil {
		t.Fatalf("CreateFiverrAccount: %v", err)
	}
	customer, err := st.GetOrCreateCustomer(ctx, account.ID, "buyer1", "Buyer One")
	if err != nil {
		t.Fatalf("GetOrCreateCustomer: %v", err)
	}
	order, err := st.CreateOrder(ctx, OrderInput{FiverrAccountID: account.ID, CustomerID: customer.ID, AmountCents: 5000})
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}

	// A different authenticated user must never be able to read another
	// seller's order by guessing its id (docs/security.md RBAC).
	if _, err := st.GetOrder(ctx, order.ID, strangerID); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for a stranger's lookup, got %v", err)
	}
	if _, err := st.GetOrder(ctx, order.ID, ownerID); err != nil {
		t.Fatalf("owner should be able to read their own order: %v", err)
	}
}
