// Package testutil provides isolated PostgreSQL schemas for integration tests.
package testutil

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

// DatabaseURL creates a unique schema and drops only that schema at cleanup.
// Tests fail explicitly when PostgreSQL is unavailable; they never skip storage.
func DatabaseURL(t testing.TB) string {
	t.Helper()
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Fatal("TEST_DATABASE_URL is required; see README.md for PostgreSQL test setup")
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		t.Fatal("TEST_DATABASE_URL must be a PostgreSQL URL")
	}
	config, err := pgx.ParseConfig(raw)
	if err != nil {
		t.Fatal("invalid TEST_DATABASE_URL")
	}
	db := stdlib.OpenDB(*config)
	var token [12]byte
	if _, err := rand.Read(token[:]); err != nil {
		db.Close()
		t.Fatal(err)
	}
	schema := "studio_test_" + hex.EncodeToString(token[:])
	quoted := pgx.Identifier{schema}.Sanitize()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA "+quoted); err != nil {
		db.Close()
		t.Fatalf("create PostgreSQL test schema: %v", err)
	}
	t.Cleanup(func() {
		defer db.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if _, err := db.ExecContext(ctx, "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Errorf("remove PostgreSQL test schema: %v", err)
		}
	})
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	return u.String()
}
