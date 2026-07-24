package migrations

import (
	"fmt"

	"goravel/app/facades"
)

// M20260724000002EnforceArtlyUUIDSchema makes the UUID change visible to
// installations that already recorded the original table migration.
type M20260724000002EnforceArtlyUUIDSchema struct{}

func (r *M20260724000002EnforceArtlyUUIDSchema) Signature() string {
	return "20260724000002_enforce_artly_uuid_schema"
}

func (r *M20260724000002EnforceArtlyUUIDSchema) Up() error {
	var incompatible []struct {
		TableName  string `db:"table_name"`
		ColumnName string `db:"column_name"`
		DataType   string `db:"data_type"`
	}

	err := facades.Schema().Orm().Query().Raw(`
		WITH expected(table_name, column_name) AS (
			VALUES
				('users', 'id'),
				('topics', 'id'),
				('topic_aliases', 'id'),
				('topic_aliases', 'topic_id'),
				('posts', 'id'),
				('posts', 'user_id'),
				('post_topics', 'post_id'),
				('post_topics', 'topic_id'),
				('reactions', 'id'),
				('reactions', 'post_id'),
				('reactions', 'user_id'),
				('messages', 'id'),
				('messages', 'sender_id'),
				('messages', 'receiver_id')
		)
		SELECT
			expected.table_name,
			expected.column_name,
			COALESCE(columns.udt_name, 'missing') AS data_type
		FROM expected
		JOIN information_schema.tables AS tables
			ON tables.table_schema = current_schema()
			AND tables.table_name = expected.table_name
			AND tables.table_type = 'BASE TABLE'
		LEFT JOIN information_schema.columns AS columns
			ON columns.table_schema = current_schema()
			AND columns.table_name = expected.table_name
			AND columns.column_name = expected.column_name
		WHERE columns.udt_name IS DISTINCT FROM 'uuid'
		ORDER BY expected.table_name, expected.column_name
		LIMIT 1
	`).Scan(&incompatible)
	if err != nil {
		return fmt.Errorf("kiểm tra schema UUID hiện có: %w", err)
	}
	if len(incompatible) > 0 {
		column := incompatible[0]
		return fmt.Errorf(
			"schema Artly cũ không tương thích: %s.%s có kiểu %s; hãy sao lưu và reset các bảng demo theo README trước khi chạy lại migration",
			column.TableName,
			column.ColumnName,
			column.DataType,
		)
	}

	// On a fresh database migration 000001 has already created the tables. If
	// an older installation kept its migration ledger after a documented reset,
	// this call recreates the current UUID schema in the same transaction.
	return (&M20260724000001CreateArtlySocialTables{}).Up()
}

// Down intentionally keeps the schema intact. Migration 000001 owns table
// removal; this compatibility guard must never delete application data itself.
func (r *M20260724000002EnforceArtlyUUIDSchema) Down() error {
	return nil
}
