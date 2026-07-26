package repositories_test

import (
	"context"
	"os"
	"testing"
	"time"

	"goravel/app/repositories"
	"goravel/bootstrap"
)

// TestLiveMessageRepositoryUsesConversationSchema is opt-in because it uses
// the configured PostgreSQL database. It only reads existing demo data.
func TestLiveMessageRepositoryUsesConversationSchema(t *testing.T) {
	if os.Getenv("MESSAGE_REPOSITORY_LIVE_TEST") != "1" {
		t.Skip("set MESSAGE_REPOSITORY_LIVE_TEST=1 to use the configured database")
	}

	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir("../.."); err != nil {
		t.Fatalf("change to backend directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(workingDirectory)
	})

	bootstrap.Boot()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	contentRepository := repositories.NewContentRepository()
	users, _, err := contentRepository.ListUsers(ctx, 1, 2)
	if err != nil {
		t.Fatalf("list live demo users: %v", err)
	}
	if len(users) < 2 {
		t.Skip("configured database needs at least two demo users")
	}

	messageRepository := repositories.NewMessageRepository()
	messages, total, err := messageRepository.ListConversation(
		ctx,
		users[0].ID,
		users[1].ID,
		1,
		50,
	)
	if err != nil {
		t.Fatalf("list live direct conversation: %v", err)
	}
	if total < int64(len(messages)) {
		t.Fatalf("total messages %d is smaller than returned page %d", total, len(messages))
	}
}
