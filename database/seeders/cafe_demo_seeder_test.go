package seeders

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestCafeDemoDataContainsFourAccountsWithOneCoffeePostEach(t *testing.T) {
	t.Parallel()

	data := cafeDemoData("https://assets.example.test/demo-art")

	if len(data.Users) != 4 {
		t.Fatalf("demo users = %d, want 4", len(data.Users))
	}
	if len(data.Posts) != 4 {
		t.Fatalf("demo posts = %d, want 4", len(data.Posts))
	}

	usersByEmail := make(map[string]cafeDemoUser, len(data.Users))
	usernames := make(map[string]struct{}, len(data.Users))
	for _, user := range data.Users {
		if user.Email == "" || user.Username == "" || user.DisplayName == "" {
			t.Fatalf("demo user has incomplete identity: %#v", user)
		}
		if user.Role != "STUDENT" && user.Role != "TEACHER" {
			t.Fatalf("demo user %s has unsupported role %q", user.Email, user.Role)
		}
		if _, exists := usersByEmail[user.Email]; exists {
			t.Fatalf("duplicate demo email %q", user.Email)
		}
		if _, exists := usernames[user.Username]; exists {
			t.Fatalf("duplicate demo username %q", user.Username)
		}
		usersByEmail[user.Email] = user
		usernames[user.Username] = struct{}{}
	}

	postIDs := make(map[string]struct{}, len(data.Posts))
	imageNames := make(map[string]struct{}, len(data.Posts))
	postCountByEmail := make(map[string]int, len(data.Users))
	for _, post := range data.Posts {
		if _, exists := usersByEmail[post.AuthorEmail]; !exists {
			t.Fatalf("post %q references unknown author %q", post.Title, post.AuthorEmail)
		}
		if _, exists := postIDs[post.ID]; exists {
			t.Fatalf("duplicate demo post id %q", post.ID)
		}
		if post.Title == "" || post.Caption == "" || post.ExamName == "" {
			t.Fatalf("demo post has incomplete copy: %#v", post)
		}
		if !strings.HasPrefix(
			post.ImageURL,
			"https://assets.example.test/demo-art/",
		) {
			t.Fatalf("post %q image URL = %q", post.Title, post.ImageURL)
		}
		if filepath.Ext(post.ImageFile) != ".webp" {
			t.Fatalf("post %q image file = %q, want WebP", post.Title, post.ImageFile)
		}
		if !strings.Contains(strings.ToLower(post.Title+" "+post.Caption), "cà phê") {
			t.Fatalf("post %q must be clearly related to coffee", post.Title)
		}
		if _, exists := imageNames[post.ImageFile]; exists {
			t.Fatalf("duplicate demo image %q", post.ImageFile)
		}

		postIDs[post.ID] = struct{}{}
		imageNames[post.ImageFile] = struct{}{}
		postCountByEmail[post.AuthorEmail]++
	}

	for _, user := range data.Users {
		if got := postCountByEmail[user.Email]; got != 1 {
			t.Errorf("posts by %s = %d, want 1", user.Email, got)
		}
	}
}

func TestCafeDemoDataUsesStableIdsAndNormalizesAssetBaseURL(t *testing.T) {
	t.Parallel()

	first := cafeDemoData("https://assets.example.test/demo-art/")
	second := cafeDemoData("https://assets.example.test/demo-art")

	if first.Topic.ID != cafeDemoTopicID || first.Topic.Slug != "ca-phe" {
		t.Fatalf("coffee topic = %#v", first.Topic)
	}
	if len(first.Posts) != len(second.Posts) {
		t.Fatalf("post counts differ: %d and %d", len(first.Posts), len(second.Posts))
	}

	for index := range first.Posts {
		if first.Posts[index].ID != second.Posts[index].ID {
			t.Errorf("post %d id is not stable", index)
		}
		if first.Posts[index].ImageURL != second.Posts[index].ImageURL {
			t.Errorf(
				"post %d image URL differs by trailing slash: %q and %q",
				index,
				first.Posts[index].ImageURL,
				second.Posts[index].ImageURL,
			)
		}
	}
}

func TestSupabaseDemoClientReusesExistingAuthUsersAndCreatesMissingOnes(
	t *testing.T,
) {
	t.Parallel()

	createdEmails := make([]string, 0, 1)
	updatedUserIDs := make([]string, 0, 1)
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		if request.Header.Get("Authorization") != "Bearer test-service-key" {
			t.Errorf("Authorization header = %q", request.Header.Get("Authorization"))
		}
		if request.Header.Get("apikey") != "test-service-key" {
			t.Errorf("apikey header = %q", request.Header.Get("apikey"))
		}

		switch {
		case request.Method == http.MethodGet &&
			request.URL.Path == "/auth/v1/admin/users":
			writer.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(
				writer,
				`{"users":[{"id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","email":"existing@example.test"}]}`,
			)
		case request.Method == http.MethodPost &&
			request.URL.Path == "/auth/v1/admin/users":
			var payload struct {
				Email        string         `json:"email"`
				Password     string         `json:"password"`
				EmailConfirm bool           `json:"email_confirm"`
				UserMetadata map[string]any `json:"user_metadata"`
			}
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Errorf("decode create-user payload: %v", err)
				http.Error(writer, "invalid payload", http.StatusBadRequest)
				return
			}
			if !payload.EmailConfirm {
				t.Error("demo auth user must be email-confirmed")
			}
			if payload.Password != defaultDemoAuthPassword {
				t.Errorf("created password = %q", payload.Password)
			}
			if payload.UserMetadata["accountType"] != "TEACHER" {
				t.Errorf("accountType = %#v", payload.UserMetadata["accountType"])
			}
			createdEmails = append(createdEmails, payload.Email)
			writer.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(
				writer,
				`{"id":"bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb","email":"new@example.test"}`,
			)
		case request.Method == http.MethodPut &&
			request.URL.Path == "/auth/v1/admin/users/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa":
			var payload struct {
				Email        string         `json:"email"`
				Password     string         `json:"password"`
				EmailConfirm bool           `json:"email_confirm"`
				UserMetadata map[string]any `json:"user_metadata"`
			}
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Errorf("decode update-user payload: %v", err)
				http.Error(writer, "invalid payload", http.StatusBadRequest)
				return
			}
			if payload.Email != "existing@example.test" {
				t.Errorf("updated email = %q", payload.Email)
			}
			if payload.Password != defaultDemoAuthPassword {
				t.Errorf("updated password = %q", payload.Password)
			}
			if !payload.EmailConfirm {
				t.Error("updated demo auth user must be email-confirmed")
			}
			if payload.UserMetadata["username"] != "existing" {
				t.Errorf("updated username = %#v", payload.UserMetadata["username"])
			}
			updatedUserIDs = append(updatedUserIDs, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
			writer.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(
				writer,
				`{"id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","email":"existing@example.test"}`,
			)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	client := newSupabaseDemoClient(
		server.URL,
		"test-service-key",
		server.Client(),
	)
	users, err := client.ensureAuthUsers(context.Background(), []cafeDemoUser{
		{
			Email:       "existing@example.test",
			Username:    "existing",
			DisplayName: "Existing User",
			Role:        "STUDENT",
		},
		{
			Email:       "new@example.test",
			Username:    "new.user",
			DisplayName: "New User",
			Role:        "TEACHER",
		},
	})
	if err != nil {
		t.Fatalf("ensureAuthUsers error: %v", err)
	}

	if got := users["existing@example.test"]; got !=
		"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa" {
		t.Errorf("existing user id = %q", got)
	}
	if got := users["new@example.test"]; got !=
		"bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb" {
		t.Errorf("new user id = %q", got)
	}
	if len(createdEmails) != 1 || createdEmails[0] != "new@example.test" {
		t.Errorf("created emails = %#v, want only new@example.test", createdEmails)
	}
	if len(updatedUserIDs) != 1 ||
		updatedUserIDs[0] != "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa" {
		t.Errorf("updated user ids = %#v", updatedUserIDs)
	}
}

func TestSupabaseDemoClientCreatesPublicBucketAndUploadsWebP(t *testing.T) {
	t.Parallel()

	var bucketCreated bool
	var uploadedBody string
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		switch {
		case request.Method == http.MethodGet &&
			request.URL.Path == "/storage/v1/bucket/demo-art":
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(writer, `{"message":"Bucket not found"}`)
		case request.Method == http.MethodPost &&
			request.URL.Path == "/storage/v1/bucket":
			var payload struct {
				ID               string   `json:"id"`
				Public           bool     `json:"public"`
				AllowedMimeTypes []string `json:"allowed_mime_types"`
			}
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Errorf("decode bucket payload: %v", err)
			}
			bucketCreated = payload.ID == "demo-art" &&
				payload.Public &&
				len(payload.AllowedMimeTypes) == 1 &&
				payload.AllowedMimeTypes[0] == "image/webp"
			writer.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(writer, `{"name":"demo-art"}`)
		case request.Method == http.MethodPost &&
			request.URL.Path == "/storage/v1/object/demo-art/coffee.webp":
			body, err := io.ReadAll(request.Body)
			if err != nil {
				t.Errorf("read upload body: %v", err)
			}
			uploadedBody = string(body)
			if request.Header.Get("Content-Type") != "image/webp" {
				t.Errorf("upload Content-Type = %q", request.Header.Get("Content-Type"))
			}
			if request.Header.Get("x-upsert") != "true" {
				t.Errorf("upload x-upsert = %q", request.Header.Get("x-upsert"))
			}
			writer.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(writer, `{"Key":"demo-art/coffee.webp"}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	client := newSupabaseDemoClient(
		server.URL,
		"test-service-key",
		server.Client(),
	)
	if err := client.ensureDemoArtBucket(context.Background()); err != nil {
		t.Fatalf("ensureDemoArtBucket error: %v", err)
	}
	if err := client.uploadDemoArtwork(
		context.Background(),
		"coffee.webp",
		[]byte("fake-webp"),
	); err != nil {
		t.Fatalf("uploadDemoArtwork error: %v", err)
	}

	if !bucketCreated {
		t.Fatal("public demo-art bucket was not created with WebP allowlist")
	}
	if uploadedBody != "fake-webp" {
		t.Errorf("uploaded body = %q", uploadedBody)
	}
}
