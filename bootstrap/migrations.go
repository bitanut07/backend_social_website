package bootstrap

import (
	"github.com/goravel/framework/contracts/database/schema"

	"goravel/database/migrations"
)

func Migrations() []schema.Migration {
	return []schema.Migration{
		&migrations.M20260724000001CreateArtlySocialTables{},
		&migrations.M20260724000002EnforceArtlyUUIDSchema{},
	}
}
