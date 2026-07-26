package migrations

import (
	"fmt"

	"goravel/app/facades"
)

// M20260726000005AddIsSuperAdminToUsers marks accounts with super-admin access.
type M20260726000005AddIsSuperAdminToUsers struct{}

func (r *M20260726000005AddIsSuperAdminToUsers) Signature() string {
	return "20260726000005_add_is_super_admin_to_users"
}

func (r *M20260726000005AddIsSuperAdminToUsers) Up() error {
	if err := facades.Schema().Sql(
		`ALTER TABLE "users"
		 ADD COLUMN IF NOT EXISTS "is_super_admin" BOOLEAN NOT NULL DEFAULT FALSE`,
	); err != nil {
		return fmt.Errorf("thêm cột is_super_admin vào bảng users: %w", err)
	}

	return nil
}

func (r *M20260726000005AddIsSuperAdminToUsers) Down() error {
	if err := facades.Schema().Sql(
		`ALTER TABLE IF EXISTS "users"
		 DROP COLUMN IF EXISTS "is_super_admin"`,
	); err != nil {
		return fmt.Errorf("xóa cột is_super_admin khỏi bảng users: %w", err)
	}

	return nil
}
