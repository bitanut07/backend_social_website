package tests

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSQLScriptRunsInPostgreSQLEditorsWithoutPsqlCommands(t *testing.T) {
	t.Parallel()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("không xác định được đường dẫn file test")
	}

	scriptPath := filepath.Join(filepath.Dir(currentFile), "..", "sql.sql")
	script, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("đọc sql.sql: %v", err)
	}

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
