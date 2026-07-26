package migrations

import (
	"fmt"

	contractsschema "github.com/goravel/framework/contracts/database/schema"
	frameworkschema "github.com/goravel/framework/database/schema"

	"goravel/app/facades"
)

type M20260724000001CreateArtlySocialTables struct{}

// Signature returns the unique migration identifier.
func (r *M20260724000001CreateArtlySocialTables) Signature() string {
	return "20260724000001_create_artly_social_tables"
}

// Up creates the Artly tables in foreign-key dependency order.
func (r *M20260724000001CreateArtlySocialTables) Up() error {
	richSchema, err := usesSupabaseSocialSchema()
	if err != nil {
		return err
	}
	if richSchema {
		return nil
	}

	if err := createUsersTable(); err != nil {
		return err
	}
	if err := createTopicsTable(); err != nil {
		return err
	}
	if err := createTopicAliasesTable(); err != nil {
		return err
	}
	if err := createPostsTable(); err != nil {
		return err
	}
	if err := createPostTopicsTable(); err != nil {
		return err
	}
	if err := createReactionsTable(); err != nil {
		return err
	}
	if err := createMessagesTable(); err != nil {
		return err
	}
	if err := ensureCheckConstraints(); err != nil {
		return err
	}

	return ensureUpdatedAtTriggers()
}

// usesSupabaseSocialSchema detects the richer Artly schema, where post media
// lives in post_media rather than posts.image_url. That schema is managed by
// Supabase migrations and must not receive constraints for the compact demo
// schema.
func usesSupabaseSocialSchema() (bool, error) {
	postsExist, err := currentSchemaHasTable("posts")
	if err != nil || !postsExist {
		return false, err
	}

	hasImageURL, err := currentSchemaHasColumn("posts", "image_url")
	if err != nil {
		return false, err
	}
	hasPostMedia, err := currentSchemaHasTable("post_media")
	if err != nil {
		return false, err
	}
	return !hasImageURL && hasPostMedia, nil
}

// Down drops dependent tables before the tables they reference.
func (r *M20260724000001CreateArtlySocialTables) Down() error {
	for _, table := range []string{
		"messages",
		"reactions",
		"post_topics",
		"posts",
		"topic_aliases",
		"topics",
		"users",
	} {
		if err := facades.Schema().DropIfExists(table); err != nil {
			return err
		}
	}

	if err := facades.Schema().Sql(`DROP FUNCTION IF EXISTS artly_set_updated_at()`); err != nil {
		return fmt.Errorf("xóa function cập nhật updated_at: %w", err)
	}

	return nil
}

func createUsersTable() error {
	return createTableIfMissing("users", func(table contractsschema.Blueprint) {
		addUUIDPrimaryKey(table)
		table.String("username", 50)
		table.String("display_name", 100)
		table.Enum("role", []any{"STUDENT", "TEACHER"})
		table.String("avatar_url", 2048).Nullable()
		addTimestamps(table)

		table.Unique("username").Name("users_username_unique")
		table.Index("role").Name("users_role_index")
	})
}

func createTopicsTable() error {
	return createTableIfMissing("topics", func(table contractsschema.Blueprint) {
		addUUIDPrimaryKey(table)
		table.String("slug", 100)
		table.String("name", 100)
		table.String("normalized_name", 100)
		addTimestamps(table)

		table.Unique("slug").Name("topics_slug_unique")
		table.Unique("normalized_name").Name("topics_normalized_name_unique")
		table.Index("name").Name("topics_name_index")
	})
}

func createTopicAliasesTable() error {
	return createTableIfMissing("topic_aliases", func(table contractsschema.Blueprint) {
		addUUIDPrimaryKey(table)
		table.Uuid("topic_id")
		table.String("alias", 100)
		table.String("normalized_alias", 100)
		addTimestamps(table)

		table.Unique("normalized_alias").Name("topic_aliases_normalized_alias_unique")
		table.Index("topic_id").Name("topic_aliases_topic_index")
		table.Foreign("topic_id").
			References("id").
			On("topics").
			Name("fk_topic_aliases_topic").
			CascadeOnUpdate().
			CascadeOnDelete()
	})
}

func createPostsTable() error {
	return createTableIfMissing("posts", func(table contractsschema.Blueprint) {
		addUUIDPrimaryKey(table)
		table.Uuid("user_id")
		table.String("title", 120)
		table.Text("caption")
		table.String("image_url", 2048)
		table.String("exam_name", 160).Nullable()
		table.Enum("status", []any{"PUBLISHED", "ARCHIVED"}).Default("PUBLISHED")
		addTimestamps(table)

		table.Index("status", "created_at", "id").Name("posts_status_created_id_index")
		table.Index("user_id", "created_at", "id").Name("posts_user_created_id_index")
		table.Foreign("user_id").
			References("id").
			On("users").
			Name("fk_posts_user").
			CascadeOnUpdate().
			RestrictOnDelete()
	})
}

func createPostTopicsTable() error {
	return createTableIfMissing("post_topics", func(table contractsschema.Blueprint) {
		addUUIDPrimaryKey(table)
		table.Uuid("post_id")
		table.Uuid("topic_id")
		addTimestamps(table)

		table.Unique("post_id", "topic_id").Name("post_topics_pair_unique")
		table.Index("topic_id", "post_id").Name("post_topics_topic_post_index")
		table.Foreign("post_id").
			References("id").
			On("posts").
			Name("fk_post_topics_post").
			CascadeOnUpdate().
			CascadeOnDelete()
		table.Foreign("topic_id").
			References("id").
			On("topics").
			Name("fk_post_topics_topic").
			CascadeOnUpdate().
			CascadeOnDelete()
	})
}

func createReactionsTable() error {
	return createTableIfMissing("reactions", func(table contractsschema.Blueprint) {
		addUUIDPrimaryKey(table)
		table.Uuid("post_id")
		table.Uuid("user_id")
		table.Enum("type", []any{"LIKE", "LOVE", "CLAP"}).Default("LIKE")
		addTimestamps(table)

		table.Unique("post_id", "user_id").Name("reactions_post_user_unique")
		table.Index("user_id", "created_at").Name("reactions_user_created_index")
		table.Foreign("post_id").
			References("id").
			On("posts").
			Name("fk_reactions_post").
			CascadeOnUpdate().
			CascadeOnDelete()
		table.Foreign("user_id").
			References("id").
			On("users").
			Name("fk_reactions_user").
			CascadeOnUpdate().
			CascadeOnDelete()
	})
}

func createMessagesTable() error {
	return createTableIfMissing("messages", func(table contractsschema.Blueprint) {
		addUUIDPrimaryKey(table)
		table.Uuid("sender_id")
		table.Uuid("receiver_id")
		table.Text("body")
		table.Boolean("is_read").Default(false)
		addTimestamps(table)

		table.Index("sender_id", "receiver_id", "created_at", "id").
			Name("messages_sender_receiver_created_index")
		table.Index("receiver_id", "sender_id", "created_at", "id").
			Name("messages_receiver_sender_created_index")
		table.Index("receiver_id", "is_read", "created_at").
			Name("messages_receiver_unread_index")
		table.Foreign("sender_id").
			References("id").
			On("users").
			Name("fk_messages_sender").
			CascadeOnUpdate().
			RestrictOnDelete()
		table.Foreign("receiver_id").
			References("id").
			On("users").
			Name("fk_messages_receiver").
			CascadeOnUpdate().
			RestrictOnDelete()
	})
}

func createTableIfMissing(
	name string,
	callback func(table contractsschema.Blueprint),
) error {
	exists, err := currentSchemaHasTable(name)
	if err != nil {
		return fmt.Errorf("kiểm tra bảng %s trong schema hiện tại: %w", name, err)
	}
	if exists {
		return nil
	}

	return facades.Schema().Create(name, callback)
}

func currentSchemaHasTable(name string) (bool, error) {
	var counts []struct {
		Total int64 `db:"total"`
	}

	err := facades.Schema().Orm().Query().Raw(
		`SELECT COUNT(*) AS total
		 FROM information_schema.tables
		 WHERE table_schema = current_schema()
		   AND table_name = ?
		   AND table_type = 'BASE TABLE'`,
		name,
	).Scan(&counts)
	if err != nil {
		return false, err
	}
	if len(counts) != 1 {
		return false, fmt.Errorf("không đọc được metadata bảng")
	}

	return counts[0].Total > 0, nil
}

func currentSchemaHasColumn(table, column string) (bool, error) {
	var counts []struct {
		Total int64 `db:"total"`
	}

	err := facades.Schema().Orm().Query().Raw(
		`SELECT COUNT(*) AS total
		 FROM information_schema.columns
		 WHERE table_schema = current_schema()
		   AND table_name = ?
		   AND column_name = ?`,
		table,
		column,
	).Scan(&counts)
	if err != nil {
		return false, err
	}
	if len(counts) != 1 {
		return false, fmt.Errorf("không đọc được metadata cột")
	}

	return counts[0].Total > 0, nil
}

func addUUIDPrimaryKey(table contractsschema.Blueprint) {
	table.Uuid("id").Default(frameworkschema.Expression("gen_random_uuid()"))
	table.Primary("id")
}

func addTimestamps(table contractsschema.Blueprint) {
	table.TimestampTz("created_at", 3).UseCurrent()
	table.TimestampTz("updated_at", 3).UseCurrent()
}

type checkConstraint struct {
	table     string
	name      string
	statement string
}

// ensureCheckConstraints fills the small gap in Blueprint: Goravel v1.18
// does not expose a CHECK helper, so these static PostgreSQL statements are
// guarded through information_schema to keep retries safe.
func ensureCheckConstraints() error {
	constraints := []checkConstraint{
		{
			table:     "users",
			name:      "chk_users_username_format",
			statement: `ALTER TABLE "users" ADD CONSTRAINT "chk_users_username_format" CHECK ("username" ~ '^[a-z0-9._-]{3,50}$')`,
		},
		{
			table:     "users",
			name:      "chk_users_display_name_length",
			statement: `ALTER TABLE "users" ADD CONSTRAINT "chk_users_display_name_length" CHECK (CHAR_LENGTH(TRIM("display_name")) BETWEEN 1 AND 100)`,
		},
		{
			table:     "users",
			name:      "chk_users_avatar_url",
			statement: `ALTER TABLE "users" ADD CONSTRAINT "chk_users_avatar_url" CHECK ("avatar_url" IS NULL OR "avatar_url" LIKE 'http://%' OR "avatar_url" LIKE 'https://%')`,
		},
		{
			table:     "topics",
			name:      "chk_topics_name_length",
			statement: `ALTER TABLE "topics" ADD CONSTRAINT "chk_topics_name_length" CHECK (CHAR_LENGTH(TRIM("name")) BETWEEN 1 AND 100)`,
		},
		{
			table:     "topics",
			name:      "chk_topics_normalized_name_length",
			statement: `ALTER TABLE "topics" ADD CONSTRAINT "chk_topics_normalized_name_length" CHECK (CHAR_LENGTH(TRIM("normalized_name")) BETWEEN 1 AND 100)`,
		},
		{
			table:     "topics",
			name:      "chk_topics_slug_format",
			statement: `ALTER TABLE "topics" ADD CONSTRAINT "chk_topics_slug_format" CHECK ("slug" ~ '^[a-z0-9]+(-[a-z0-9]+)*$')`,
		},
		{
			table:     "topic_aliases",
			name:      "chk_topic_aliases_length",
			statement: `ALTER TABLE "topic_aliases" ADD CONSTRAINT "chk_topic_aliases_length" CHECK (CHAR_LENGTH(TRIM("alias")) BETWEEN 1 AND 100 AND CHAR_LENGTH(TRIM("normalized_alias")) BETWEEN 1 AND 100)`,
		},
		{
			table:     "posts",
			name:      "chk_posts_title_length",
			statement: `ALTER TABLE "posts" ADD CONSTRAINT "chk_posts_title_length" CHECK (CHAR_LENGTH(TRIM("title")) BETWEEN 1 AND 120)`,
		},
		{
			table:     "posts",
			name:      "chk_posts_caption_length",
			statement: `ALTER TABLE "posts" ADD CONSTRAINT "chk_posts_caption_length" CHECK (CHAR_LENGTH(TRIM("caption")) BETWEEN 1 AND 2000)`,
		},
		{
			table:     "posts",
			name:      "chk_posts_image_url",
			statement: `ALTER TABLE "posts" ADD CONSTRAINT "chk_posts_image_url" CHECK ("image_url" LIKE 'http://%' OR "image_url" LIKE 'https://%')`,
		},
		{
			table:     "posts",
			name:      "chk_posts_exam_name_length",
			statement: `ALTER TABLE "posts" ADD CONSTRAINT "chk_posts_exam_name_length" CHECK ("exam_name" IS NULL OR CHAR_LENGTH(TRIM("exam_name")) BETWEEN 1 AND 160)`,
		},
		{
			table:     "messages",
			name:      "chk_messages_body_length",
			statement: `ALTER TABLE "messages" ADD CONSTRAINT "chk_messages_body_length" CHECK (CHAR_LENGTH(TRIM("body")) BETWEEN 1 AND 2000)`,
		},
		{
			table:     "messages",
			name:      "chk_messages_is_read",
			statement: `ALTER TABLE "messages" ADD CONSTRAINT "chk_messages_is_read" CHECK ("is_read" IN (FALSE, TRUE))`,
		},
	}

	for _, constraint := range constraints {
		exists, err := checkConstraintExists(constraint.table, constraint.name)
		if err != nil {
			return fmt.Errorf("kiểm tra constraint %s: %w", constraint.name, err)
		}
		if exists {
			continue
		}
		if err := facades.Schema().Sql(constraint.statement); err != nil {
			return fmt.Errorf("tạo constraint %s: %w", constraint.name, err)
		}
	}

	return nil
}

func ensureUpdatedAtTriggers() error {
	return ensureUpdatedAtTriggersForTables(
		"users",
		"topics",
		"topic_aliases",
		"posts",
		"post_topics",
		"reactions",
		"messages",
	)
}

func ensureUpdatedAtTriggersForTables(tables ...string) error {
	const createFunctionStatement = `CREATE OR REPLACE FUNCTION artly_set_updated_at()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  NEW.updated_at := CURRENT_TIMESTAMP;
  RETURN NEW;
END;
$$`

	if err := facades.Schema().Sql(createFunctionStatement); err != nil {
		return fmt.Errorf("tạo hoặc cập nhật function cập nhật updated_at: %w", err)
	}

	for _, table := range tables {
		trigger := table + "_set_updated_at"

		if err := facades.Schema().Sql(fmt.Sprintf(
			`DROP TRIGGER IF EXISTS "%s" ON "%s"`,
			trigger,
			table,
		)); err != nil {
			return fmt.Errorf("xóa trigger %s: %w", trigger, err)
		}

		if err := facades.Schema().Sql(fmt.Sprintf(
			`CREATE TRIGGER "%s"
BEFORE UPDATE ON "%s"
FOR EACH ROW
EXECUTE FUNCTION artly_set_updated_at()`,
			trigger,
			table,
		)); err != nil {
			return fmt.Errorf("tạo trigger %s: %w", trigger, err)
		}
	}

	return nil
}

func checkConstraintExists(table, name string) (bool, error) {
	var counts []struct {
		Total int64 `db:"total"`
	}

	err := facades.Schema().Orm().Query().Raw(
		`SELECT COUNT(*) AS total
		 FROM information_schema.table_constraints
		 WHERE constraint_schema = current_schema()
		   AND table_name = ?
		   AND constraint_name = ?
		   AND constraint_type = 'CHECK'`,
		table,
		name,
	).Scan(&counts)
	if err != nil {
		return false, err
	}
	if len(counts) != 1 {
		return false, fmt.Errorf("không đọc được metadata constraint")
	}

	return counts[0].Total > 0, nil
}
