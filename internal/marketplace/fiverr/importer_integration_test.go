package fiverr

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lakhans7/eventbasedhttp/internal/db"
	"github.com/lakhans7/eventbasedhttp/internal/store"
)

// setupTestAccount mirrors internal/store's own test helper; duplicated
// (rather than imported) because it needs a fiverr_account owned by a real
// user, and pulling internal/store's test-only helpers across package
// boundaries isn't possible in Go. See docs/security.md for why malformed
// rows must never abort an entire import.
func setupTestAccount(t *testing.T) (*store.Store, string) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://postgres:postgres@localhost:5432/fiverr_saas_test?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Skipf("skipping: no test database reachable at %s: %v", url, err)
	}
	t.Cleanup(pool.Close)

	migrateCtx, migrateCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer migrateCancel()
	if err := db.Migrate(migrateCtx, pool, "../../../migrations"); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}

	st := store.New(pool)
	var userID string
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO users (email, name, status) VALUES ($1, 'Import Tester', 'active') RETURNING id
	`, "importer-"+time.Now().Format("150405.000000")+"@example.com").Scan(&userID); err != nil {
		t.Fatalf("create test user: %v", err)
	}
	account, err := st.CreateFiverrAccount(context.Background(), userID, "importer_test")
	if err != nil {
		t.Fatalf("CreateFiverrAccount: %v", err)
	}
	return st, account.ID
}

func TestImportGigsSkipsMalformedRowsWithoutAbortingImport(t *testing.T) {
	st, accountID := setupTestAccount(t)

	csvData := "title,base_price_cents,status\n" +
		"Good gig,100,active\n" +
		",50,active\n" + // missing required title -> skipped
		"Bad price gig,not-a-number,active\n" + // invalid price -> skipped
		"Another good gig,200,paused\n"

	result, err := ImportGigs(context.Background(), st, accountID, strings.NewReader(csvData))
	if err != nil {
		t.Fatalf("ImportGigs: %v", err)
	}
	if result.Imported != 2 {
		t.Errorf("expected 2 successful imports, got %d (skipped: %+v)", result.Imported, result.Skipped)
	}
	if len(result.Skipped) != 2 {
		t.Errorf("expected 2 skipped rows, got %d", len(result.Skipped))
	}

	gigs, err := st.ListGigs(context.Background(), mustGetOwner(t, st, accountID), accountID)
	if err != nil {
		t.Fatalf("ListGigs: %v", err)
	}
	if len(gigs) != 2 {
		t.Fatalf("expected 2 gigs persisted, got %d", len(gigs))
	}
}

func mustGetOwner(t *testing.T, st *store.Store, accountID string) string {
	t.Helper()
	var userID string
	if err := st.Pool.QueryRow(context.Background(), `SELECT user_id FROM fiverr_accounts WHERE id = $1`, accountID).Scan(&userID); err != nil {
		t.Fatalf("lookup account owner: %v", err)
	}
	return userID
}
