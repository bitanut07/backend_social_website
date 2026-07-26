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
	if err := ensureArtlyIdentifierColumnsAreUUID(); err != nil {
		return err
	}

	richSchema, err := usesSupabaseSocialSchema()
	if err != nil {
		return err
	}
	if richSchema {
		return nil
	}

	var incompatible []struct {
		TableName  string `db:"table_name"`
		ColumnName string `db:"column_name"`
		DataType   string `db:"data_type"`
	}

	err = facades.Schema().Orm().Query().Raw(`
		WITH expected(table_name, column_name) AS (
			VALUES
				('users', 'id'),
				('topics', 'id'),
				('topic_aliases', 'id'),
				('topic_aliases', 'topic_id'),
				('posts', 'id'),
				('posts', 'user_id'),
				('post_topics', 'id'),
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

type incompatibleIdentifierColumn struct {
	TableName  string `db:"table_name"`
	ColumnName string `db:"column_name"`
	DataType   string `db:"data_type"`
}

func ensureArtlyIdentifierColumnsAreUUID() error {
	return CheckArtlyIdentifierColumnsAreUUID()
}

// CheckArtlyIdentifierColumnsAreUUID verifies that every Artly identifier
// column in the configured schema is a UUID.
func CheckArtlyIdentifierColumnsAreUUID() error {
	var incompatible []incompatibleIdentifierColumn
	if err := facades.Schema().Orm().Query().Raw(uuidIdentifierAuditSQL).Scan(&incompatible); err != nil {
		return fmt.Errorf("kiểm tra các cột định danh UUID: %w", err)
	}
	if len(incompatible) == 0 {
		return nil
	}

	column := incompatible[0]
	return fmt.Errorf(
		"schema Artly không tương thích: %s.%s có kiểu %s; mọi cột id/*_id phải là uuid",
		column.TableName,
		column.ColumnName,
		column.DataType,
	)
}

const uuidIdentifierAuditSQL = `
	WITH expected(table_name, column_name) AS (
		VALUES
			('users', 'id'),
			('follows', 'id'),
			('follows', 'follower_id'),
			('follows', 'following_id'),
			('user_blocks', 'id'),
			('user_blocks', 'blocker_id'),
			('user_blocks', 'blocked_id'),
			('posts', 'id'),
			('posts', 'user_id'),
			('post_media', 'id'),
			('post_media', 'post_id'),
			('hashtags', 'id'),
			('post_hashtags', 'id'),
			('post_hashtags', 'post_id'),
			('post_hashtags', 'hashtag_id'),
			('topics', 'id'),
			('topic_aliases', 'id'),
			('topic_aliases', 'topic_id'),
			('post_topics', 'id'),
			('post_topics', 'post_id'),
			('post_topics', 'topic_id'),
			('saved_posts', 'id'),
			('saved_posts', 'user_id'),
			('saved_posts', 'post_id'),
			('post_shares', 'id'),
			('post_shares', 'post_id'),
			('post_shares', 'user_id'),
			('contests', 'id'),
			('contests', 'organizer_id'),
			('contest_topics', 'id'),
			('contest_topics', 'contest_id'),
			('contest_topics', 'topic_id'),
			('contest_submissions', 'id'),
			('contest_submissions', 'contest_id'),
			('contest_submissions', 'post_id'),
			('contest_submissions', 'submitted_by'),
			('contest_submissions', 'reviewed_by'),
			('reaction_types', 'id'),
			('post_reactions', 'id'),
			('post_reactions', 'post_id'),
			('post_reactions', 'user_id'),
			('post_reactions', 'reaction_type_id'),
			('comments', 'id'),
			('comments', 'post_id'),
			('comments', 'user_id'),
			('comments', 'parent_comment_id'),
			('comment_reactions', 'id'),
			('comment_reactions', 'comment_id'),
			('comment_reactions', 'user_id'),
			('comment_reactions', 'reaction_type_id'),
			('conversations', 'id'),
			('conversations', 'created_by'),
			('direct_conversation_pairs', 'id'),
			('direct_conversation_pairs', 'conversation_id'),
			('direct_conversation_pairs', 'user_low_id'),
			('direct_conversation_pairs', 'user_high_id'),
			('conversation_participants', 'id'),
			('conversation_participants', 'conversation_id'),
			('conversation_participants', 'user_id'),
			('messages', 'id'),
			('messages', 'sender_id'),
			('message_attachments', 'id'),
			('message_attachments', 'message_id'),
			('message_reads', 'id'),
			('message_reads', 'message_id'),
			('message_reads', 'user_id'),
			('notifications', 'id'),
			('notifications', 'user_id'),
			('notifications', 'actor_id'),
			('notifications', 'entity_id'),
			('reports', 'id'),
			('reports', 'reported_by'),
			('reports', 'target_id'),
			('reports', 'handled_by'),
			('assistant_conversations', 'id'),
			('assistant_conversations', 'user_id'),
			('assistant_messages', 'id'),
			('assistant_messages', 'conversation_id'),
			('assistant_message_post_refs', 'id'),
			('assistant_message_post_refs', 'assistant_message_id'),
			('assistant_message_post_refs', 'post_id'),
			('assistant_query_logs', 'id'),
			('assistant_query_logs', 'user_id'),
			('assistant_query_logs', 'conversation_id'),
			('assistant_chat_conversations', 'id'),
			('assistant_chat_conversations', 'user_id'),
			('assistant_chat_messages', 'id'),
			('assistant_chat_messages', 'conversation_id'),
			('reactions', 'id'),
			('reactions', 'post_id'),
			('reactions', 'user_id')
	),
	expected_violations AS (
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
	),
	discovered_violations AS (
		SELECT
			columns.table_name,
			columns.column_name,
			columns.udt_name AS data_type
		FROM information_schema.columns AS columns
		JOIN information_schema.tables AS tables
			ON tables.table_schema = columns.table_schema
			AND tables.table_name = columns.table_name
			AND tables.table_type = 'BASE TABLE'
		WHERE columns.table_schema = current_schema()
			AND columns.table_name NOT IN (
				'artly_goravel_migrations',
				'auth_identities',
				'auth_tokens',
				'migrations',
				'schema_migrations',
				'supabase_migrations',
				'user_sessions'
			)
			AND (
				columns.column_name = 'id'
				OR right(columns.column_name, 3) = '_id'
				OR columns.column_name IN (
					'submitted_by',
					'reviewed_by',
					'reported_by',
					'handled_by',
					'created_by'
				)
			)
			AND columns.udt_name <> 'uuid'
	)
	SELECT table_name, column_name, data_type
	FROM (
		SELECT * FROM expected_violations
		UNION
		SELECT * FROM discovered_violations
	) violations
	ORDER BY table_name, column_name
	LIMIT 1
`
