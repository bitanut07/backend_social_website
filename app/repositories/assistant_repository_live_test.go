package repositories_test

import (
	"context"
	"os"
	"testing"
	"time"

	"goravel/app/facades"
	"goravel/app/repositories"
	"goravel/bootstrap"
)

// TestLiveAssistantRepository is opt-in because it uses the configured
// PostgreSQL database. It logs no credentials or connection strings.
func TestLiveAssistantRepository(t *testing.T) {
	if os.Getenv("ASSISTANT_REPOSITORY_LIVE_TEST") != "1" {
		t.Skip("set ASSISTANT_REPOSITORY_LIVE_TEST=1 to use the configured database")
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
	repository := repositories.NewAssistantRepository(facades.Orm())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	topic, found, err := repository.ResolveTopic(ctx, "ca phe")
	if err != nil {
		t.Fatalf("ResolveTopic(ca phe) live error: %v", err)
	}
	if !found || topic.Slug != "ca-phe" {
		t.Fatalf("ResolveTopic(ca phe) = (%#v, %v)", topic, found)
	}

	count, err := repository.CountPublishedPostsByTopic(ctx, topic.ID)
	if err != nil {
		t.Fatalf("CountPublishedPostsByTopic() live error: %v", err)
	}
	t.Logf("live topic=%q count=%d", topic.Name, count)
}
