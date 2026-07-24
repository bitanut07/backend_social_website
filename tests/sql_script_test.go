package tests

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSQLScriptRunsInPostgreSQLEditorsWithoutPsqlCommands(t *testing.T) {
	t.Parallel()

	script := readSQLScript(t)

	scanner := bufio.NewScanner(bytes.NewReader(script))
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, `\`) {
			t.Fatalf(
				"sql.sql dòng %d chứa psql meta-command %q; Supabase SQL Editor chỉ nhận SQL thuần",
				lineNumber,
				line,
			)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("đọc từng dòng sql.sql: %v", err)
	}

	if strings.Contains(strings.ToUpper(string(script)), "CREATE DATABASE") {
		t.Fatal("sql.sql không được tự tạo database khi chạy trong Supabase SQL Editor")
	}
}

func TestSQLScriptUsesUUIDKeysWithoutSequences(t *testing.T) {
	t.Parallel()

	script := string(readSQLScript(t))
	upperScript := strings.ToUpper(script)

	for _, forbidden := range []string{
		"BIGINT",
		"IDENTITY",
		"BIGSERIAL",
		"PG_GET_SERIAL_SEQUENCE",
		"SETVAL(",
		"NEXTVAL(",
	} {
		if strings.Contains(upperScript, forbidden) {
			t.Errorf("sql.sql còn cấu trúc khóa tuần tự không được phép: %s", forbidden)
		}
	}

	resourceTables := []string{
		"users",
		"topics",
		"topic_aliases",
		"posts",
		"reactions",
		"messages",
	}
	for _, table := range resourceTables {
		table := table
		t.Run(table, func(t *testing.T) {
			ddl := sqlSection(
				t,
				script,
				"CREATE TABLE IF NOT EXISTS "+table+" (",
				"\n);",
			)
			if !strings.Contains(ddl, "id UUID NOT NULL DEFAULT gen_random_uuid()") {
				t.Errorf("bảng %s phải dùng UUID tự sinh cho khóa chính", table)
			}
		})
	}

	if got := strings.Count(script, "DEFAULT gen_random_uuid()"); got != len(resourceTables) {
		t.Errorf(
			"số khóa chính có DEFAULT gen_random_uuid() = %d, muốn %d",
			got,
			len(resourceTables),
		)
	}

	foreignKeys := map[string][]string{
		"topic_aliases": {"topic_id"},
		"posts":         {"user_id"},
		"post_topics":   {"post_id", "topic_id"},
		"reactions":     {"post_id", "user_id"},
		"messages":      {"sender_id", "receiver_id"},
	}
	for table, columns := range foreignKeys {
		ddl := sqlSection(
			t,
			script,
			"CREATE TABLE IF NOT EXISTS "+table+" (",
			"\n);",
		)
		for _, column := range columns {
			if !strings.Contains(ddl, column+" UUID NOT NULL") {
				t.Errorf("bảng %s: khóa ngoại %s phải dùng UUID NOT NULL", table, column)
			}
		}
	}
}

func TestSQLScriptRejectsAnExistingNonUUIDSchemaEarly(t *testing.T) {
	t.Parallel()

	script := string(readSQLScript(t))
	preflight := sqlSection(
		t,
		script,
		"DO $$",
		"CREATE TABLE IF NOT EXISTS users",
	)
	for _, expected := range []string{
		"LEFT JOIN information_schema.columns",
		"columns.udt_name IS DISTINCT FROM 'uuid'",
		"Schema Artly hiện có không tương thích",
	} {
		if !strings.Contains(preflight, expected) {
			t.Errorf("sql.sql thiếu preflight kiểm tra schema UUID: %q", expected)
		}
	}

	resourceKeys := map[string][]string{
		"users":         {"id"},
		"topics":        {"id"},
		"topic_aliases": {"id", "topic_id"},
		"posts":         {"id", "user_id"},
		"post_topics":   {"post_id", "topic_id"},
		"reactions":     {"id", "post_id", "user_id"},
		"messages":      {"id", "sender_id", "receiver_id"},
	}
	for table, columns := range resourceKeys {
		for _, column := range columns {
			expected := fmt.Sprintf("('%s', '%s')", table, column)
			if !strings.Contains(preflight, expected) {
				t.Errorf("preflight thiếu khóa UUID %s.%s", table, column)
			}
		}
	}
}

func TestSQLScriptContainsStableUUIDSeeds(t *testing.T) {
	t.Parallel()

	script := string(readSQLScript(t))
	seedGroups := []struct {
		table      string
		prefix     string
		count      int
		start, end string
	}{
		{"users", "00000000", 3, "INSERT INTO users", "INSERT INTO topics"},
		{"topics", "10000000", 6, "INSERT INTO topics", "INSERT INTO topic_aliases"},
		{
			"topic_aliases",
			"30000000",
			12,
			"INSERT INTO topic_aliases",
			"INSERT INTO posts",
		},
		{"posts", "20000000", 5, "INSERT INTO posts", "INSERT INTO post_topics"},
		{"reactions", "40000000", 6, "INSERT INTO reactions", "INSERT INTO messages"},
		{"messages", "50000000", 4, "INSERT INTO messages", "COMMIT;"},
	}

	for _, group := range seedGroups {
		section := sqlSection(t, script, group.start, group.end)
		for index := 1; index <= group.count; index++ {
			expected := stableUUID(group.prefix, index)
			if got := strings.Count(section, "'"+expected+"'"); got != 1 {
				t.Errorf(
					"seed %s phải chứa UUID chính %s đúng một lần, nhận %d",
					group.table,
					expected,
					got,
				)
			}
		}
	}
}

func TestSQLScriptKeepsUUIDSeedRelationships(t *testing.T) {
	t.Parallel()

	script := string(readSQLScript(t))

	aliases := sqlSection(
		t,
		script,
		"INSERT INTO topic_aliases",
		"INSERT INTO posts",
	)
	for alias := 1; alias <= 12; alias++ {
		topic := (alias + 1) / 2
		assertTuplePrefix(
			t,
			aliases,
			stableUUID("30000000", alias),
			stableUUID("10000000", topic),
		)
	}

	posts := sqlSection(t, script, "INSERT INTO posts", "INSERT INTO post_topics")
	for post, user := range []int{1, 3, 1, 3, 1} {
		assertTuplePrefix(
			t,
			posts,
			stableUUID("20000000", post+1),
			stableUUID("00000000", user),
		)
	}

	postTopics := sqlSection(
		t,
		script,
		"INSERT INTO post_topics",
		"INSERT INTO reactions",
	)
	for _, relation := range [][2]int{
		{1, 1},
		{1, 3},
		{2, 4},
		{3, 5},
		{4, 2},
		{5, 1},
		{5, 6},
	} {
		assertTuplePrefix(
			t,
			postTopics,
			stableUUID("20000000", relation[0]),
			stableUUID("10000000", relation[1]),
		)
	}

	reactions := sqlSection(
		t,
		script,
		"INSERT INTO reactions",
		"INSERT INTO messages",
	)
	reactionPosts := []int{1, 1, 2, 3, 4, 5}
	reactionUsers := []int{2, 3, 1, 2, 1, 2}
	for index := range reactionPosts {
		assertTuplePrefix(
			t,
			reactions,
			stableUUID("40000000", index+1),
			stableUUID("20000000", reactionPosts[index]),
			stableUUID("00000000", reactionUsers[index]),
		)
	}

	messages := sqlSection(t, script, "INSERT INTO messages", "COMMIT;")
	messageSenders := []int{1, 2, 3, 2}
	messageReceivers := []int{2, 1, 2, 3}
	for index := range messageSenders {
		assertTuplePrefix(
			t,
			messages,
			stableUUID("50000000", index+1),
			stableUUID("00000000", messageSenders[index]),
			stableUUID("00000000", messageReceivers[index]),
		)
	}
}

func TestSQLScriptRerunsAfterAReactionWasRecreated(t *testing.T) {
	t.Parallel()

	reactions := sqlSection(
		t,
		string(readSQLScript(t)),
		"INSERT INTO reactions",
		"INSERT INTO messages",
	)
	if !strings.Contains(
		reactions,
		"ON CONFLICT (post_id, user_id) DO UPDATE SET",
	) {
		t.Fatal(
			"reaction seed phải xử lý unique post_id/user_id để chạy lại sau thao tác reaction",
		)
	}
}

func readSQLScript(t *testing.T) []byte {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("không xác định được đường dẫn file test")
	}

	scriptPath := filepath.Join(filepath.Dir(currentFile), "..", "sql.sql")
	script, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("đọc sql.sql: %v", err)
	}

	return script
}

func sqlSection(t *testing.T, script, startMarker, endMarker string) string {
	t.Helper()

	start := strings.Index(script, startMarker)
	if start < 0 {
		t.Fatalf("sql.sql thiếu marker bắt đầu %q", startMarker)
	}

	remaining := script[start:]
	end := strings.Index(remaining, endMarker)
	if end < 0 {
		t.Fatalf("sql.sql thiếu marker kết thúc %q sau %q", endMarker, startMarker)
	}

	return remaining[:end]
}

func stableUUID(prefix string, index int) string {
	return fmt.Sprintf("%s-0000-4000-8000-%012d", prefix, index)
}

func assertTuplePrefix(t *testing.T, section string, values ...string) {
	t.Helper()

	quotedValues := make([]string, 0, len(values))
	for _, value := range values {
		quotedValues = append(quotedValues, "'"+value+"'")
	}

	normalized := strings.Join(strings.Fields(section), " ")
	expected := "( " + strings.Join(quotedValues, ", ")
	if got := strings.Count(normalized, expected); got != 1 {
		t.Errorf("seed phải chứa tuple bắt đầu bằng %q đúng một lần, nhận %d", expected, got)
	}
}
