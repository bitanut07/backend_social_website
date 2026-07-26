package seeders

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/goravel/framework/contracts/database/db"

	"goravel/app/facades"
)

const demoArtBucket = "demo-art"

type CafeDemoSeeder struct{}

func (s *CafeDemoSeeder) Signature() string {
	return "CafeDemoSeeder"
}

func (s *CafeDemoSeeder) Run() error {
	supabaseURL := strings.TrimRight(
		facades.Config().GetString("supabase.url"),
		"/",
	)
	serviceKey := strings.TrimSpace(
		facades.Config().GetString("supabase.secret_key"),
	)
	if supabaseURL == "" || serviceKey == "" {
		return fmt.Errorf(
			"cần cấu hình SUPABASE_URL và SUPABASE_SECRET_KEY để seed dữ liệu cà phê",
		)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	client := newSupabaseDemoClient(supabaseURL, serviceKey, nil)
	if err := client.ensureDemoArtBucket(ctx); err != nil {
		return err
	}

	data := cafeDemoData(
		supabaseURL + "/storage/v1/object/public/" + demoArtBucket,
	)
	assetSizes, err := uploadCafeDemoAssets(ctx, client, data.Posts)
	if err != nil {
		return err
	}

	userIDs, err := client.ensureAuthUsers(ctx, data.Users)
	if err != nil {
		return err
	}

	return facades.DB().WithContext(ctx).Transaction(func(tx db.Tx) error {
		if err := seedCafeDemoProfiles(tx, data.Users, userIDs); err != nil {
			return err
		}
		if err := seedCafeDemoTopic(tx, data.Topic); err != nil {
			return err
		}
		if err := seedCafeDemoPosts(
			tx,
			data.Posts,
			userIDs,
			assetSizes,
		); err != nil {
			return err
		}
		if err := seedCafeDemoReactions(tx, data.Reactions, userIDs); err != nil {
			return err
		}

		return nil
	})
}

func uploadCafeDemoAssets(
	ctx context.Context,
	client *supabaseDemoClient,
	posts []cafeDemoPost,
) (map[string]int64, error) {
	assetDir := strings.TrimSpace(os.Getenv("ARTLY_DEMO_ASSET_DIR"))
	if assetDir == "" {
		assetDir = filepath.Join("..", "frontend", "public", "demo-art")
	}

	sizes := make(map[string]int64, len(posts))
	for _, post := range posts {
		path := filepath.Join(assetDir, post.ImageFile)
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("đọc ảnh demo %s: %w", path, err)
		}
		if !isWebP(content) {
			return nil, fmt.Errorf("ảnh demo %s không phải WebP hợp lệ", path)
		}
		if err = client.uploadDemoArtwork(ctx, post.ImageFile, content); err != nil {
			return nil, err
		}
		sizes[post.ImageFile] = int64(len(content))
	}

	return sizes, nil
}

func seedCafeDemoProfiles(
	tx db.Tx,
	users []cafeDemoUser,
	userIDs map[string]string,
) error {
	for _, user := range users {
		userID := userIDs[strings.ToLower(user.Email)]
		if userID == "" {
			return fmt.Errorf("thiếu ID Supabase cho tài khoản %s", user.Email)
		}

		_, err := tx.Table("users").UpdateOrInsert(
			map[string]any{"id": userID},
			map[string]any{
				"username":      user.Username,
				"primary_email": user.Email,
				"display_name":  user.DisplayName,
				"bio":           user.Bio,
				"avatar_url":    nil,
				"role":          user.Role,
				"status":        "ACTIVE",
				"is_private":    false,
				"deleted_at":    nil,
			},
		)
		if err != nil {
			return fmt.Errorf("seed hồ sơ %s: %w", user.Email, err)
		}
	}

	return nil
}

func seedCafeDemoTopic(tx db.Tx, topic cafeDemoTopic) error {
	_, err := tx.Table("topics").UpdateOrInsert(
		map[string]any{"id": topic.ID},
		map[string]any{
			"slug":            topic.Slug,
			"name":            topic.Name,
			"normalized_name": topic.Normalized,
			"description":     topic.Description,
		},
	)
	if err != nil {
		return fmt.Errorf("seed chủ đề cà phê: %w", err)
	}

	return nil
}

func seedCafeDemoPosts(
	tx db.Tx,
	posts []cafeDemoPost,
	userIDs map[string]string,
	assetSizes map[string]int64,
) error {
	for _, post := range posts {
		userID := userIDs[strings.ToLower(post.AuthorEmail)]
		if userID == "" {
			return fmt.Errorf("thiếu tác giả cho bài %s", post.Title)
		}

		publishedAt := time.Now().
			UTC().
			Add(-time.Duration(post.PublishedAgoH) * time.Hour)
		_, err := tx.Table("posts").UpdateOrInsert(
			map[string]any{"id": post.ID},
			map[string]any{
				"user_id":        userID,
				"title":          post.Title,
				"caption":        post.Caption,
				"exam_name":      post.ExamName,
				"visibility":     "PUBLIC",
				"status":         "PUBLISHED",
				"comment_policy": "EVERYONE",
				"published_at":   publishedAt,
				"deleted_at":     nil,
			},
		)
		if err != nil {
			return fmt.Errorf("seed bài %s: %w", post.Title, err)
		}

		_, err = tx.Table("post_media").UpdateOrInsert(
			map[string]any{
				"post_id":  post.ID,
				"position": 0,
			},
			map[string]any{
				"media_type":         "IMAGE",
				"storage_bucket":     demoArtBucket,
				"storage_path":       post.ImageFile,
				"media_url":          post.ImageURL,
				"alt_text":           post.Title,
				"mime_type":          "image/webp",
				"size_bytes":         assetSizes[post.ImageFile],
				"width":              1254,
				"height":             1254,
				"original_file_name": post.ImageFile,
			},
		)
		if err != nil {
			return fmt.Errorf("seed ảnh bài %s: %w", post.Title, err)
		}

		_, err = tx.Table("post_topics").UpdateOrInsert(
			map[string]any{
				"post_id":  post.ID,
				"topic_id": cafeDemoTopicID,
			},
			map[string]any{
				"source":     "SYSTEM",
				"confidence": 1,
			},
		)
		if err != nil {
			return fmt.Errorf("gắn chủ đề cho bài %s: %w", post.Title, err)
		}
	}

	return nil
}

func seedCafeDemoReactions(
	tx db.Tx,
	reactions []cafeDemoReaction,
	userIDs map[string]string,
) error {
	for _, reaction := range reactions {
		userID := userIDs[strings.ToLower(reaction.ReactorEmail)]
		if userID == "" {
			return fmt.Errorf(
				"thiếu người thả reaction %s",
				reaction.ReactorEmail,
			)
		}

		_, err := tx.Table("post_reactions").UpdateOrInsert(
			map[string]any{
				"post_id": reaction.PostID,
				"user_id": userID,
			},
			map[string]any{
				"reaction_type_id": reaction.ReactionTypeID,
			},
		)
		if err != nil {
			return fmt.Errorf(
				"seed reaction cho bài %s: %w",
				reaction.PostID,
				err,
			)
		}
	}

	return nil
}

func isWebP(content []byte) bool {
	return len(content) >= 12 &&
		bytes.Equal(content[:4], []byte("RIFF")) &&
		bytes.Equal(content[8:12], []byte("WEBP"))
}
