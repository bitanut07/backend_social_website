package migrations

// M20260725000004EnforceUUIDIdentifiers makes the UUID identifier contract
// explicit for databases that already recorded the earlier compatibility guard.
type M20260725000004EnforceUUIDIdentifiers struct{}

func (r *M20260725000004EnforceUUIDIdentifiers) Signature() string {
	return "20260725000004_enforce_uuid_identifiers"
}

func (r *M20260725000004EnforceUUIDIdentifiers) Up() error {
	return ensureArtlyIdentifierColumnsAreUUID()
}

// Down is intentionally a no-op because this migration only audits schema
// safety; it never owns table removal or type changes.
func (r *M20260725000004EnforceUUIDIdentifiers) Down() error {
	return nil
}
