package repositories_test

import (
	"context"
	"os"
	"testing"
	"time"

	"goravel/app/repositories"
	"goravel/bootstrap"
)

// TestLiveContentRepositoryReactionRoundTrip is opt-in because it briefly
// changes one demo reaction, then restores its original state.
func TestLiveContentRepositoryReactionRoundTrip(t *testing.T) {
	if os.Getenv("CONTENT_REPOSITORY_REACTION_LIVE_TEST") != "1" {
		t.Skip("set CONTENT_REPOSITORY_REACTION_LIVE_TEST=1 to use the configured database")
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
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	repository := repositories.NewContentRepository()
	users, _, err := repository.ListUsers(ctx, 1, 1)
	if err != nil {
		t.Fatalf("list live demo users: %v", err)
	}
	if len(users) == 0 {
		t.Skip("configured database needs at least one demo user")
	}

	posts, _, err := repository.ListPosts(ctx, users[0].ID, 1, 1, nil, nil)
	if err != nil {
		t.Fatalf("list live posts: %v", err)
	}
	if len(posts) == 0 {
		t.Skip("configured database needs at least one published post")
	}

	post := posts[0]
	restore := func() {
		restoreCtx, restoreCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer restoreCancel()
		if post.ViewerHasReacted {
			_, _ = repository.PutReaction(restoreCtx, users[0].ID, post.ID)
		} else {
			_, _ = repository.DeleteReaction(restoreCtx, users[0].ID, post.ID)
		}
	}
	t.Cleanup(restore)

	if post.ViewerHasReacted {
		state, deleteErr := repository.DeleteReaction(ctx, users[0].ID, post.ID)
		if deleteErr != nil {
			t.Fatalf("delete live reaction: %v", deleteErr)
		}
		if state.ViewerHasReacted {
			t.Fatal("delete reaction returned viewerHasReacted=true")
		}

		state, putErr := repository.PutReaction(ctx, users[0].ID, post.ID)
		if putErr != nil {
			t.Fatalf("restore live reaction: %v", putErr)
		}
		if !state.ViewerHasReacted {
			t.Fatal("put reaction returned viewerHasReacted=false")
		}
		return
	}

	state, err := repository.PutReaction(ctx, users[0].ID, post.ID)
	if err != nil {
		t.Fatalf("put live reaction: %v", err)
	}
	if !state.ViewerHasReacted {
		t.Fatal("put reaction returned viewerHasReacted=false")
	}

	state, err = repository.DeleteReaction(ctx, users[0].ID, post.ID)
	if err != nil {
		t.Fatalf("delete live reaction: %v", err)
	}
	if state.ViewerHasReacted {
		t.Fatal("delete reaction returned viewerHasReacted=true")
	}
}
