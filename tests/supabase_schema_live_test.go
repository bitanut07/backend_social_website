package tests

import (
	"os"
	"testing"

	"goravel/bootstrap"
	"goravel/database/migrations"
)

// TestLiveSupabaseIdentifierColumnsAreUUID is opt-in because it connects to
// the configured Supabase PostgreSQL database. It logs no credentials or
// connection strings.
func TestLiveSupabaseIdentifierColumnsAreUUID(t *testing.T) {
	if os.Getenv("SUPABASE_SCHEMA_LIVE_TEST") != "1" {
		t.Skip("set SUPABASE_SCHEMA_LIVE_TEST=1 to audit the configured Supabase schema")
	}

	bootstrap.Boot()
	if err := migrations.CheckArtlyIdentifierColumnsAreUUID(); err != nil {
		t.Fatalf("Supabase UUID identifier audit failed: %v", err)
	}
}
