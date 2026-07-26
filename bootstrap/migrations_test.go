package bootstrap

import "testing"

func TestMigrationsIncludeAssistantChatHistory(t *testing.T) {
	t.Parallel()

	migrations := Migrations()
	if len(migrations) != 5 {
		t.Fatalf("Migrations() trả %d migration, muốn 5", len(migrations))
	}

	if got := migrations[0].Signature(); got != "20260724000001_create_artly_social_tables" {
		t.Fatalf("migration đầu tiên = %q", got)
	}
	if got := migrations[1].Signature(); got != "20260724000002_enforce_artly_uuid_schema" {
		t.Fatalf("migration UUID tương thích = %q", got)
	}
	if got := migrations[2].Signature(); got != "20260725000003_create_assistant_chat_history" {
		t.Fatalf("migration lịch sử chat = %q", got)
	}
	if got := migrations[3].Signature(); got != "20260725000004_enforce_uuid_identifiers" {
		t.Fatalf("migration guard UUID định danh = %q", got)
	}
	if got := migrations[4].Signature(); got != "20260726000005_add_is_super_admin_to_users" {
		t.Fatalf("migration super admin = %q", got)
	}
}
