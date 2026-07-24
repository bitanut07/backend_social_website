package bootstrap

import "testing"

func TestMigrationsIncludeUUIDCompatibilityGuard(t *testing.T) {
	t.Parallel()

	migrations := Migrations()
	if len(migrations) != 2 {
		t.Fatalf("Migrations() trả %d migration, muốn 2", len(migrations))
	}

	if got := migrations[0].Signature(); got != "20260724000001_create_artly_social_tables" {
		t.Fatalf("migration đầu tiên = %q", got)
	}
	if got := migrations[1].Signature(); got != "20260724000002_enforce_artly_uuid_schema" {
		t.Fatalf("migration UUID tương thích = %q", got)
	}
}
