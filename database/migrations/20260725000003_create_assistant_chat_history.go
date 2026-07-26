package migrations

import (
	"fmt"

	contractsschema "github.com/goravel/framework/contracts/database/schema"

	"goravel/app/facades"
)

// M20260725000003CreateAssistantChatHistory adds persistent, user-owned AI chats.
type M20260725000003CreateAssistantChatHistory struct{}

func (r *M20260725000003CreateAssistantChatHistory) Signature() string {
	return "20260725000003_create_assistant_chat_history"
}

func (r *M20260725000003CreateAssistantChatHistory) Up() error {
	if err := createAssistantConversationsTable(); err != nil {
		return err
	}
	if err := createAssistantMessagesTable(); err != nil {
		return err
	}
	if err := ensureAssistantHistoryConstraints(); err != nil {
		return err
	}

	return ensureUpdatedAtTriggersForTables(
		"assistant_chat_conversations",
		"assistant_chat_messages",
	)
}

func (r *M20260725000003CreateAssistantChatHistory) Down() error {
	for _, table := range []string{"assistant_chat_messages", "assistant_chat_conversations"} {
		if err := facades.Schema().DropIfExists(table); err != nil {
			return err
		}
	}
	return nil
}

func createAssistantConversationsTable() error {
	return createTableIfMissing(
		"assistant_chat_conversations",
		func(table contractsschema.Blueprint) {
			addUUIDPrimaryKey(table)
			table.Uuid("user_id")
			table.String("title", 80)
			addTimestamps(table)

			table.Index("user_id", "updated_at", "id").
				Name("assistant_chat_conversations_user_updated_id_index")
			table.Foreign("user_id").
				References("id").
				On("users").
				Name("fk_assistant_chat_conversations_user").
				CascadeOnUpdate().
				CascadeOnDelete()
		},
	)
}

func createAssistantMessagesTable() error {
	return createTableIfMissing(
		"assistant_chat_messages",
		func(table contractsschema.Blueprint) {
			addUUIDPrimaryKey(table)
			table.Uuid("conversation_id")
			table.Integer("position")
			table.Enum("role", []any{"USER", "ASSISTANT"})
			table.Text("content")
			table.Jsonb("response_json").Nullable()
			addTimestamps(table)

			table.Unique("conversation_id", "position").
				Name("assistant_chat_messages_conversation_position_unique")
			table.Foreign("conversation_id").
				References("id").
				On("assistant_chat_conversations").
				Name("fk_assistant_chat_messages_conversation").
				CascadeOnUpdate().
				CascadeOnDelete()
		},
	)
}

func ensureAssistantHistoryConstraints() error {
	constraints := []checkConstraint{
		{
			table:     "assistant_chat_conversations",
			name:      "chk_assistant_chat_conversations_title_length",
			statement: `ALTER TABLE "assistant_chat_conversations" ADD CONSTRAINT "chk_assistant_chat_conversations_title_length" CHECK (CHAR_LENGTH(TRIM("title")) BETWEEN 1 AND 80)`,
		},
		{
			table:     "assistant_chat_messages",
			name:      "chk_assistant_chat_messages_position",
			statement: `ALTER TABLE "assistant_chat_messages" ADD CONSTRAINT "chk_assistant_chat_messages_position" CHECK ("position" > 0)`,
		},
		{
			table: "assistant_chat_messages",
			name:  "chk_assistant_chat_messages_payload",
			statement: `ALTER TABLE "assistant_chat_messages" ADD CONSTRAINT "chk_assistant_chat_messages_payload" CHECK (
				("role" = 'USER' AND CHAR_LENGTH(TRIM("content")) BETWEEN 1 AND 500 AND "response_json" IS NULL)
				OR
				("role" = 'ASSISTANT' AND CHAR_LENGTH(TRIM("content")) BETWEEN 1 AND 2000 AND "response_json" IS NOT NULL)
			)`,
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
