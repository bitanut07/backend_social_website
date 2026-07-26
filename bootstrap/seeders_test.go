package bootstrap

import "testing"

func TestSeedersRegistersCafeDemoSeeder(t *testing.T) {
	t.Parallel()

	registered := Seeders()
	if len(registered) != 1 {
		t.Fatalf("registered seeders = %d, want 1", len(registered))
	}
	if got := registered[0].Signature(); got != "CafeDemoSeeder" {
		t.Fatalf("seeder signature = %q, want CafeDemoSeeder", got)
	}
}
