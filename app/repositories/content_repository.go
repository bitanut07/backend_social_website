package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/goravel/framework/contracts/database/db"

	"goravel/app/facades"
	"goravel/app/models"
)

var ErrNotFound = errors.New("không tìm thấy tài nguyên")

var errReactionStateNotPersisted = errors.New("không thể lưu trạng thái reaction")

type User struct {
	ID          int64   `json:"id"`
	Username    string  `json:"username"`
	DisplayName string  `json:"displayName"`
	Role        string  `json:"role"`
	AvatarURL   *string `json:"avatarUrl"`
}

type Topic struct {
	ID      int64    `json:"id"`
	Slug    string   `json:"slug"`
	Name    string   `json:"name"`
	Aliases []string `json:"aliases"`
}

type Post struct {
	ID               int64     `json:"id"`
	Title            string    `json:"title"`
	Caption          string    `json:"caption"`
	ImageURL         string    `json:"imageUrl"`
	ExamName         *string   `json:"examName,omitempty"`
	Author           User      `json:"author"`
	Topics           []Topic   `json:"topics"`
	ReactionCount    int64     `json:"reactionCount"`
	ViewerHasReacted bool      `json:"viewerHasReacted"`
	CreatedAt        time.Time `json:"createdAt"`
}

type CreatePostInput struct {
	Title    string
	Caption  string
	ImageURL string
	ExamName *string
	TopicIDs []int64
}

type ReactionState struct {
	ReactionCount    int64 `json:"reactionCount"`
	ViewerHasReacted bool  `json:"viewerHasReacted"`
}

type ContentRepository interface {
	ListUsers(ctx context.Context, page, pageSize int) ([]User, int64, error)
	ListTopics(ctx context.Context, page, pageSize int) ([]Topic, int64, error)
	UserExists(ctx context.Context, userID int64) (bool, error)
	TopicExists(ctx context.Context, topicID int64) (bool, error)
	MissingTopicIDs(ctx context.Context, topicIDs []int64) ([]int64, error)
	PostExists(ctx context.Context, postID int64) (bool, error)
	ListPosts(
		ctx context.Context,
		viewerID int64,
		page, pageSize int,
		topicID *int64,
	) ([]Post, int64, error)
	CreatePost(ctx context.Context, userID int64, input CreatePostInput) (Post, error)
	PutReaction(ctx context.Context, userID, postID int64) (ReactionState, error)
	DeleteReaction(ctx context.Context, userID, postID int64) (ReactionState, error)
}

type GoravelContentRepository struct {
	database db.DB
}

func NewContentRepository(database ...db.DB) *GoravelContentRepository {
	if len(database) > 0 && database[0] != nil {
		return &GoravelContentRepository{database: database[0]}
	}

	return &GoravelContentRepository{database: facades.DB()}
}

func (r *GoravelContentRepository) ListUsers(
	ctx context.Context,
	page, pageSize int,
) ([]User, int64, error) {
	rows := make([]userRow, 0)
	var total int64

	err := r.database.WithContext(ctx).
		Table("users").
		Select("id", "username", "display_name", "role", "avatar_url").
		OrderBy("id").
		Paginate(page, pageSize, &rows, &total)
	if err != nil {
		return nil, 0, err
	}

	users := make([]User, 0, len(rows))
	for _, row := range rows {
		users = append(users, row.toUser())
	}

	return users, total, nil
}

func (r *GoravelContentRepository) ListTopics(
	ctx context.Context,
	page, pageSize int,
) ([]Topic, int64, error) {
	database := r.database.WithContext(ctx)
	rows := make([]topicRow, 0)
	var total int64

	err := database.
		Table("topics").
		Select("id", "slug", "name").
		OrderBy("id").
		Paginate(page, pageSize, &rows, &total)
	if err != nil {
		return nil, 0, err
	}

	topics, err := hydrateTopics(database, rows)
	if err != nil {
		return nil, 0, err
	}

	return topics, total, nil
}

func (r *GoravelContentRepository) UserExists(ctx context.Context, userID int64) (bool, error) {
	return r.database.WithContext(ctx).
		Table("users").
		Where("id", userID).
		Exists()
}

func (r *GoravelContentRepository) TopicExists(ctx context.Context, topicID int64) (bool, error) {
	return r.database.WithContext(ctx).
		Table("topics").
		Where("id", topicID).
		Exists()
}

func (r *GoravelContentRepository) MissingTopicIDs(
	ctx context.Context,
	topicIDs []int64,
) ([]int64, error) {
	if len(topicIDs) == 0 {
		return []int64{}, nil
	}

	var existing []int64
	err := r.database.WithContext(ctx).
		Table("topics").
		WhereIn("id", int64Values(topicIDs)).
		Pluck("id", &existing)
	if err != nil {
		return nil, err
	}

	existingSet := make(map[int64]struct{}, len(existing))
	for _, id := range existing {
		existingSet[id] = struct{}{}
	}

	missing := make([]int64, 0)
	for _, id := range topicIDs {
		if _, ok := existingSet[id]; !ok {
			missing = append(missing, id)
		}
	}

	return missing, nil
}

func (r *GoravelContentRepository) PostExists(ctx context.Context, postID int64) (bool, error) {
	return publishedPostExists(r.database.WithContext(ctx), postID)
}

func (r *GoravelContentRepository) ListPosts(
	ctx context.Context,
	viewerID int64,
	page, pageSize int,
	topicID *int64,
) ([]Post, int64, error) {
	database := r.database.WithContext(ctx)
	query := postBaseQuery(database).
		Where("posts.status", models.PostStatusPublished).
		OrderByDesc("posts.created_at").
		OrderByDesc("posts.id")
	if topicID != nil {
		query = query.
			Join("post_topics AS selected_topic ON selected_topic.post_id = posts.id").
			Where("selected_topic.topic_id", *topicID)
	}

	rows := make([]postRow, 0)
	var total int64
	if err := query.Paginate(page, pageSize, &rows, &total); err != nil {
		return nil, 0, err
	}

	posts, err := hydratePosts(database, rows, viewerID)
	if err != nil {
		return nil, 0, err
	}

	return posts, total, nil
}

func (r *GoravelContentRepository) CreatePost(
	ctx context.Context,
	userID int64,
	input CreatePostInput,
) (Post, error) {
	var created Post

	err := r.database.WithContext(ctx).Transaction(func(tx db.Tx) error {
		postID, err := tx.Table("posts").InsertGetID(map[string]any{
			"user_id":   userID,
			"title":     input.Title,
			"caption":   input.Caption,
			"image_url": input.ImageURL,
			"exam_name": input.ExamName,
			"status":    models.PostStatusPublished,
		})
		if err != nil {
			return err
		}

		postTopics := make([]map[string]any, 0, len(input.TopicIDs))
		for _, topicID := range input.TopicIDs {
			postTopics = append(postTopics, map[string]any{
				"post_id":  postID,
				"topic_id": topicID,
			})
		}
		if _, err = tx.Table("post_topics").Insert(postTopics); err != nil {
			return err
		}

		created, err = getPost(tx, userID, postID)
		return err
	})
	if err != nil {
		return Post{}, err
	}

	return created, nil
}

func (r *GoravelContentRepository) PutReaction(
	ctx context.Context,
	userID, postID int64,
) (ReactionState, error) {
	state := ReactionState{}

	err := r.database.WithContext(ctx).Transaction(func(tx db.Tx) error {
		exists, err := publishedPostExistsForUpdate(tx, postID)
		if err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}

		if err = putReactionIdempotently(tx, userID, postID); err != nil {
			return err
		}

		count, err := tx.Table("reactions").
			Where("post_id", postID).
			Count()
		if err != nil {
			return err
		}
		state = ReactionState{
			ReactionCount:    count,
			ViewerHasReacted: true,
		}

		return nil
	})
	if err != nil {
		return ReactionState{}, err
	}

	return state, nil
}

func (r *GoravelContentRepository) DeleteReaction(
	ctx context.Context,
	userID, postID int64,
) (ReactionState, error) {
	state := ReactionState{}

	err := r.database.WithContext(ctx).Transaction(func(tx db.Tx) error {
		exists, err := publishedPostExistsForUpdate(tx, postID)
		if err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}

		_, err = tx.Table("reactions").
			Where("post_id", postID).
			Where("user_id", userID).
			Delete()
		if err != nil {
			return err
		}

		count, err := tx.Table("reactions").
			Where("post_id", postID).
			Count()
		if err != nil {
			return err
		}
		state = ReactionState{
			ReactionCount:    count,
			ViewerHasReacted: false,
		}

		return nil
	})
	if err != nil {
		return ReactionState{}, err
	}

	return state, nil
}

type userRow struct {
	ID          int64   `db:"id"`
	Username    string  `db:"username"`
	DisplayName string  `db:"display_name"`
	Role        string  `db:"role"`
	AvatarURL   *string `db:"avatar_url"`
}

func (r userRow) toUser() User {
	return User{
		ID:          r.ID,
		Username:    r.Username,
		DisplayName: r.DisplayName,
		Role:        r.Role,
		AvatarURL:   r.AvatarURL,
	}
}

type topicRow struct {
	ID   int64  `db:"id"`
	Slug string `db:"slug"`
	Name string `db:"name"`
}

type aliasRow struct {
	TopicID int64  `db:"topic_id"`
	Alias   string `db:"alias"`
}

type postRow struct {
	ID                int64     `db:"id"`
	Title             string    `db:"title"`
	Caption           string    `db:"caption"`
	ImageURL          string    `db:"image_url"`
	ExamName          *string   `db:"exam_name"`
	CreatedAt         time.Time `db:"created_at"`
	AuthorID          int64     `db:"author_id"`
	AuthorUsername    string    `db:"author_username"`
	AuthorDisplayName string    `db:"author_display_name"`
	AuthorRole        string    `db:"author_role"`
	AuthorAvatarURL   *string   `db:"author_avatar_url"`
}

type postTopicRow struct {
	PostID int64  `db:"post_id"`
	ID     int64  `db:"id"`
	Slug   string `db:"slug"`
	Name   string `db:"name"`
}

type reactionCountRow struct {
	PostID int64 `db:"post_id"`
	Total  int64 `db:"total"`
}

func hydrateTopics(source db.Tx, rows []topicRow) ([]Topic, error) {
	topics := make([]Topic, 0, len(rows))
	if len(rows) == 0 {
		return topics, nil
	}

	topicIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		topicIDs = append(topicIDs, row.ID)
	}

	aliases, err := loadAliases(source, topicIDs)
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		topics = append(topics, Topic{
			ID:      row.ID,
			Slug:    row.Slug,
			Name:    row.Name,
			Aliases: aliases[row.ID],
		})
	}

	return topics, nil
}

func loadAliases(source db.Tx, topicIDs []int64) (map[int64][]string, error) {
	aliasesByTopic := make(map[int64][]string, len(topicIDs))
	for _, topicID := range topicIDs {
		aliasesByTopic[topicID] = []string{}
	}
	if len(topicIDs) == 0 {
		return aliasesByTopic, nil
	}

	rows := make([]aliasRow, 0)
	err := source.Table("topic_aliases").
		Select("topic_id", "alias").
		WhereIn("topic_id", int64Values(topicIDs)).
		OrderBy("topic_id").
		OrderBy("id").
		Get(&rows)
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		aliasesByTopic[row.TopicID] = append(aliasesByTopic[row.TopicID], row.Alias)
	}

	return aliasesByTopic, nil
}

func postBaseQuery(source db.Tx) db.Query {
	return source.Table("posts").
		Join("users ON users.id = posts.user_id").
		Select(
			"posts.id AS id",
			"posts.title AS title",
			"posts.caption AS caption",
			"posts.image_url AS image_url",
			"posts.exam_name AS exam_name",
			"posts.created_at AS created_at",
			"users.id AS author_id",
			"users.username AS author_username",
			"users.display_name AS author_display_name",
			"users.role AS author_role",
			"users.avatar_url AS author_avatar_url",
		)
}

func getPost(source db.Tx, viewerID, postID int64) (Post, error) {
	rows := make([]postRow, 0, 1)
	err := postBaseQuery(source).
		Where("posts.id", postID).
		Where("posts.status", models.PostStatusPublished).
		Limit(1).
		Get(&rows)
	if err != nil {
		return Post{}, err
	}
	if len(rows) == 0 {
		return Post{}, ErrNotFound
	}

	posts, err := hydratePosts(source, rows, viewerID)
	if err != nil {
		return Post{}, err
	}

	return posts[0], nil
}

func hydratePosts(source db.Tx, rows []postRow, viewerID int64) ([]Post, error) {
	posts := make([]Post, 0, len(rows))
	if len(rows) == 0 {
		return posts, nil
	}

	postIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		postIDs = append(postIDs, row.ID)
	}

	topicsByPost, err := loadPostTopics(source, postIDs)
	if err != nil {
		return nil, err
	}
	reactionCounts, err := loadReactionCounts(source, postIDs)
	if err != nil {
		return nil, err
	}
	viewerReactions, err := loadViewerReactions(source, postIDs, viewerID)
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		_, hasReacted := viewerReactions[row.ID]
		posts = append(posts, Post{
			ID:        row.ID,
			Title:     row.Title,
			Caption:   row.Caption,
			ImageURL:  row.ImageURL,
			ExamName:  row.ExamName,
			CreatedAt: row.CreatedAt,
			Author: User{
				ID:          row.AuthorID,
				Username:    row.AuthorUsername,
				DisplayName: row.AuthorDisplayName,
				Role:        row.AuthorRole,
				AvatarURL:   row.AuthorAvatarURL,
			},
			Topics:           topicsByPost[row.ID],
			ReactionCount:    reactionCounts[row.ID],
			ViewerHasReacted: hasReacted,
		})
	}

	return posts, nil
}

func loadPostTopics(source db.Tx, postIDs []int64) (map[int64][]Topic, error) {
	topicsByPost := make(map[int64][]Topic, len(postIDs))
	for _, postID := range postIDs {
		topicsByPost[postID] = []Topic{}
	}

	rows := make([]postTopicRow, 0)
	err := source.Table("post_topics").
		Join("topics ON topics.id = post_topics.topic_id").
		Select(
			"post_topics.post_id AS post_id",
			"topics.id AS id",
			"topics.slug AS slug",
			"topics.name AS name",
		).
		WhereIn("post_topics.post_id", int64Values(postIDs)).
		OrderBy("post_topics.post_id").
		OrderBy("topics.id").
		Get(&rows)
	if err != nil {
		return nil, err
	}

	topicIDs := make([]int64, 0, len(rows))
	seenTopics := make(map[int64]struct{}, len(rows))
	for _, row := range rows {
		if _, ok := seenTopics[row.ID]; !ok {
			seenTopics[row.ID] = struct{}{}
			topicIDs = append(topicIDs, row.ID)
		}
	}
	aliases, err := loadAliases(source, topicIDs)
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		topicsByPost[row.PostID] = append(topicsByPost[row.PostID], Topic{
			ID:      row.ID,
			Slug:    row.Slug,
			Name:    row.Name,
			Aliases: aliases[row.ID],
		})
	}

	return topicsByPost, nil
}

func loadReactionCounts(source db.Tx, postIDs []int64) (map[int64]int64, error) {
	counts := make(map[int64]int64, len(postIDs))
	rows := make([]reactionCountRow, 0)

	err := source.Table("reactions").
		Select("post_id", "COUNT(*) AS total").
		WhereIn("post_id", int64Values(postIDs)).
		GroupBy("post_id").
		Get(&rows)
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		counts[row.PostID] = row.Total
	}

	return counts, nil
}

func loadViewerReactions(
	source db.Tx,
	postIDs []int64,
	viewerID int64,
) (map[int64]struct{}, error) {
	var reactedPostIDs []int64
	err := source.Table("reactions").
		Where("user_id", viewerID).
		WhereIn("post_id", int64Values(postIDs)).
		Pluck("post_id", &reactedPostIDs)
	if err != nil {
		return nil, err
	}

	reactions := make(map[int64]struct{}, len(reactedPostIDs))
	for _, postID := range reactedPostIDs {
		reactions[postID] = struct{}{}
	}

	return reactions, nil
}

func publishedPostExists(source db.Tx, postID int64) (bool, error) {
	return source.Table("posts").
		Where("id", postID).
		Where("status", models.PostStatusPublished).
		Exists()
}

func publishedPostExistsForUpdate(source db.Tx, postID int64) (bool, error) {
	return source.Table("posts").
		Where("id", postID).
		Where("status", models.PostStatusPublished).
		LockForUpdate().
		Exists()
}

func putReactionIdempotently(source db.Tx, userID, postID int64) error {
	identity := map[string]any{
		"post_id": postID,
		"user_id": userID,
	}

	var mutationErr error
	for range 2 {
		_, mutationErr = source.Table("reactions").UpdateOrInsert(identity, identity)

		exists, existsErr := source.Table("reactions").
			Where(identity).
			LockForUpdate().
			Exists()
		if existsErr != nil {
			if mutationErr != nil {
				return errors.Join(mutationErr, existsErr)
			}
			return existsErr
		}
		if exists {
			return nil
		}
	}

	if mutationErr != nil {
		return mutationErr
	}
	return errReactionStateNotPersisted
}

func int64Values(values []int64) []any {
	result := make([]any, len(values))
	for index, value := range values {
		result[index] = value
	}

	return result
}
