package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/lakhans7/eventbasedhttp/internal/db"
)

// setupTestPool connects to a real Postgres instance for round-trip tests.
// docs/security.md explicitly calls for database tests; but this repository
// must also be runnable by a contributor with no local Postgres, so the test
// skips (rather than fails) when no database is reachable. CI (see
// .github/workflows/ci.yml) always has one, so these tests run there.
func setupTestPool(t *testing.T) *Store {
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
	if err := db.Migrate(migrateCtx, pool, "../../migrations"); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}

	return New(pool)
}

// createTestUserAndAccount inserts the minimum rows needed to satisfy the
// foreign keys under a fiverr_account (users -> fiverr_accounts), without
// depending on internal/auth (which would be a heavier, unnecessary import
// for a store-layer test).
func createTestUser(t *testing.T, st *Store) string {
	t.Helper()
	var userID string
	err := st.Pool.QueryRow(context.Background(), `
		INSERT INTO users (email, name, status) VALUES ($1, 'Test Seller', 'active') RETURNING id
	`, "test-"+randSuffix()+"@example.com").Scan(&userID)
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}
	return userID
}

var suffixCounter int

func randSuffix() string {
	suffixCounter++
	return time.Now().Format("150405") + "-" + string(rune('a'+suffixCounter%26))
}
