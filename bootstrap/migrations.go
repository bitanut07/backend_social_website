package bootstrap

import (
	"github.com/goravel/framework/contracts/database/schema"

	"goravel/database/migrations"
)

func Migrations() []schema.Migration {
	return []schema.Migration{
		&migrations.M20260724000001CreateArtlySocialTables{},
		&migrations.M20260724000002EnforceArtlyUUIDSchema{},
		&migrations.M20260725000003CreateAssistantChatHistory{},
		&migrations.M20260725000004EnforceUUIDIdentifiers{},
		&migrations.M20260726000005AddIsSuperAdminToUsers{},
	}
}
