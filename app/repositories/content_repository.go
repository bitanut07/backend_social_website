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

var ErrForbidden = errors.New("không có quyền thực hiện thao tác")

var errReactionStateNotPersisted = errors.New("không thể lưu trạng thái reaction")

const insertPostReturningIDSQL = `INSERT INTO posts (
	user_id,
	title,
	caption,
	exam_name,
	status,
	published_at
) VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP)
RETURNING id`

const defaultLikeReactionTypeID = "10000000-0000-4000-8000-000000000001"

type User struct {
	ID          string  `json:"id"`
	Username    string  `json:"username"`
	DisplayName string  `json:"displayName"`
	Role        string  `json:"role"`
	AvatarURL   *string `json:"avatarUrl"`
}

type Topic struct {
	ID      string   `json:"id"`
	Slug    string   `json:"slug"`
	Name    string   `json:"name"`
	Aliases []string `json:"aliases"`
}

type Post struct {
	ID               string    `json:"id"`
	Title            string    `json:"title"`
	Caption          string    `json:"caption"`
	ImageURL         string    `json:"imageUrl"`
	ExamName         *string   `json:"examName,omitempty"`
	Author           User      `json:"author"`
	Topics           []Topic   `json:"topics"`
	ReactionCount    int64     `json:"reactionCount"`
	CommentCount     int64     `json:"commentCount"`
	ViewerHasReacted bool      `json:"viewerHasReacted"`
	CreatedAt        time.Time `json:"createdAt"`
}

type CreatePostInput struct {
	Title    string
	Caption  string
	ImageURL string
	ExamName *string
	TopicIDs []string
}

type UpdateProfileInput struct {
	Username    string
	DisplayName string
	AvatarURL   *string
}

type ReactionState struct {
	ReactionCount    int64 `json:"reactionCount"`
	ViewerHasReacted bool  `json:"viewerHasReacted"`
}

type ContentRepository interface {
	ListUsers(ctx context.Context, page, pageSize int) ([]User, int64, error)
	UserByUsername(ctx context.Context, username string) (User, error)
	UpdateProfile(ctx context.Context, userID string, input UpdateProfileInput) (User, error)
	ListTopics(ctx context.Context, page, pageSize int) ([]Topic, int64, error)
	UserExists(ctx context.Context, userID string) (bool, error)
	TopicExists(ctx context.Context, topicID string) (bool, error)
	MissingTopicIDs(ctx context.Context, topicIDs []string) ([]string, error)
	PostExists(ctx context.Context, postID string) (bool, error)
	ListPosts(
		ctx context.Context,
		viewerID string,
		page, pageSize int,
		topicID, authorID *string,
	) ([]Post, int64, error)
	CreatePost(ctx context.Context, userID string, input CreatePostInput) (Post, error)
	DeletePost(ctx context.Context, userID, postID string) error
	PutReaction(ctx context.Context, userID, postID string) (ReactionState, error)
	DeleteReaction(ctx context.Context, userID, postID string) (ReactionState, error)
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

	database := r.database.WithContext(ctx)
	count, err := database.Table("users").Count()
	if err != nil {
		return nil, 0, err
	}
	total = count
	err = database.
		Table("users").
		Select("id", "username", "display_name", "role", "avatar_url").
		OrderBy("id").
		Offset(pageOffset(page, pageSize)).
		Limit(uint64(pageSize)).
		Get(&rows)
	if err != nil {
		return nil, 0, err
	}

	users := make([]User, 0, len(rows))
	for _, row := range rows {
		users = append(users, row.toUser())
	}

	return users, total, nil
}

func (r *GoravelContentRepository) UserByUsername(ctx context.Context, username string) (User, error) {
	rows := make([]userRow, 0, 1)
	err := r.database.WithContext(ctx).
		Table("users").
		Select("id", "username", "display_name", "role", "avatar_url").
		Where("username", username).
		Limit(1).
		Get(&rows)
	if err != nil {
		return User{}, err
	}
	if len(rows) == 0 {
		return User{}, ErrNotFound
	}

	return rows[0].toUser(), nil
}

func (r *GoravelContentRepository) UpdateProfile(
	ctx context.Context,
	userID string,
	input UpdateProfileInput,
) (User, error) {
	updated := User{}

	err := r.database.WithContext(ctx).Transaction(func(tx db.Tx) error {
		result, err := tx.Table("users").
			Where("id", userID).
			Update(map[string]any{
				"username":     input.Username,
				"display_name": input.DisplayName,
				"avatar_url":   input.AvatarURL,
				"updated_at":   time.Now().UTC(),
			})
		if err != nil {
			return err
		}
		if result == nil || result.RowsAffected != 1 {
			return ErrNotFound
		}

		rows := make([]userRow, 0, 1)
		err = tx.Table("users").
			Select("id", "username", "display_name", "role", "avatar_url").
			Where("id", userID).
			Limit(1).
			Get(&rows)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return ErrNotFound
		}

		updated = rows[0].toUser()
		return nil
	})
	if err != nil {
		return User{}, err
	}

	return updated, nil
}

func (r *GoravelContentRepository) ListTopics(
	ctx context.Context,
	page, pageSize int,
) ([]Topic, int64, error) {
	database := r.database.WithContext(ctx)
	rows := make([]topicRow, 0)
	var total int64

	count, err := database.Table("topics").Count()
	if err != nil {
		return nil, 0, err
	}
	total = count
	err = database.
		Table("topics").
		Select("id", "slug", "name").
		OrderBy("id").
		Offset(pageOffset(page, pageSize)).
		Limit(uint64(pageSize)).
		Get(&rows)
	if err != nil {
		return nil, 0, err
	}

	topics, err := hydrateTopics(database, rows)
	if err != nil {
		return nil, 0, err
	}

	return topics, total, nil
}

func (r *GoravelContentRepository) UserExists(ctx context.Context, userID string) (bool, error) {
	return r.database.WithContext(ctx).
		Table("users").
		Where("id", userID).
		Exists()
}

func (r *GoravelContentRepository) TopicExists(ctx context.Context, topicID string) (bool, error) {
	return r.database.WithContext(ctx).
		Table("topics").
		Where("id", topicID).
		Exists()
}

func (r *GoravelContentRepository) MissingTopicIDs(
	ctx context.Context,
	topicIDs []string,
) ([]string, error) {
	if len(topicIDs) == 0 {
		return []string{}, nil
	}

	var existing []string
	err := r.database.WithContext(ctx).
		Table("topics").
		WhereIn("id", stringValues(topicIDs)).
		Pluck("id", &existing)
	if err != nil {
		return nil, err
	}

	existingSet := make(map[string]struct{}, len(existing))
	for _, id := range existing {
		existingSet[id] = struct{}{}
	}

	missing := make([]string, 0)
	for _, id := range topicIDs {
		if _, ok := existingSet[id]; !ok {
			missing = append(missing, id)
		}
	}

	return missing, nil
}

func (r *GoravelContentRepository) PostExists(ctx context.Context, postID string) (bool, error) {
	return publishedPostExists(r.database.WithContext(ctx), postID)
}

func (r *GoravelContentRepository) ListPosts(
	ctx context.Context,
	viewerID string,
	page, pageSize int,
	topicID, authorID *string,
) ([]Post, int64, error) {
	database := r.database.WithContext(ctx)
	countQuery := database.
		Table("posts").
		Join("post_media AS media ON media.post_id = posts.id AND media.position = 0").
		Where("posts.status", models.PostStatusPublished).
		WhereNull("posts.deleted_at")
	if topicID != nil {
		countQuery = countQuery.
			Join("post_topics AS selected_topic ON selected_topic.post_id = posts.id").
			Where("selected_topic.topic_id", *topicID)
	}
	if authorID != nil {
		countQuery = countQuery.Where("posts.user_id", *authorID)
	}
	total, err := countQuery.Count()
	if err != nil {
		return nil, 0, err
	}

	query := postBaseQuery(database).
		Where("posts.status", models.PostStatusPublished).
		WhereNull("posts.deleted_at")
	if topicID != nil {
		query = query.
			Join("post_topics AS selected_topic ON selected_topic.post_id = posts.id").
			Where("selected_topic.topic_id", *topicID)
	}
	if authorID != nil {
		query = query.Where("posts.user_id", *authorID)
	}

	rows := make([]postRow, 0)
	err = query.
		OrderByDesc("posts.created_at").
		OrderByDesc("posts.id").
		Offset(pageOffset(page, pageSize)).
		Limit(uint64(pageSize)).
		Get(&rows)
	if err != nil {
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
	userID string,
	input CreatePostInput,
) (Post, error) {
	var created Post

	err := r.database.WithContext(ctx).Transaction(func(tx db.Tx) error {
		postID, err := insertPostReturningID(tx, userID, input)
		if err != nil {
			return err
		}

		_, err = tx.Table("post_media").Insert(map[string]any{
			"post_id":    postID,
			"media_type": "IMAGE",
			"media_url":  input.ImageURL,
			"position":   0,
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

func (r *GoravelContentRepository) DeletePost(
	ctx context.Context,
	userID, postID string,
) error {
	return r.database.WithContext(ctx).Transaction(func(tx db.Tx) error {
		owners := make([]postOwnerRow, 0, 1)
		err := tx.Table("posts").
			Select("user_id").
			Where("id", postID).
			WhereNull("deleted_at").
			LockForUpdate().
			Limit(1).
			Get(&owners)
		if err != nil {
			return err
		}
		if len(owners) == 0 {
			return ErrNotFound
		}
		if owners[0].UserID != userID {
			return ErrForbidden
		}

		result, err := tx.Table("posts").
			Where("id", postID).
			Where("user_id", userID).
			WhereNull("deleted_at").
			Update(map[string]any{
				"status":     models.PostStatusRemoved,
				"deleted_at": time.Now().UTC(),
			})
		if err != nil {
			return err
		}
		if result == nil || result.RowsAffected != 1 {
			return ErrNotFound
		}

		return nil
	})
}

func insertPostReturningID(source db.Tx, userID string, input CreatePostInput) (string, error) {
	row := insertedPostIDRow{}
	err := source.Select(
		&row,
		insertPostReturningIDSQL,
		userID,
		input.Title,
		input.Caption,
		input.ExamName,
		models.PostStatusPublished,
	)
	if err != nil {
		return "", err
	}

	return row.ID, nil
}

func (r *GoravelContentRepository) PutReaction(
	ctx context.Context,
	userID, postID string,
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

		count, err := tx.Table("post_reactions").
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
	userID, postID string,
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

		_, err = tx.Table("post_reactions").
			Where("post_id", postID).
			Where("user_id", userID).
			Delete()
		if err != nil {
			return err
		}

		count, err := tx.Table("post_reactions").
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
	ID          string  `db:"id"`
	Username    string  `db:"username"`
	DisplayName string  `db:"display_name"`
	Role        string  `db:"role"`
	AvatarURL   *string `db:"avatar_url"`
}

type insertedPostIDRow struct {
	ID string `db:"id"`
}

type lockedResourceRow struct {
	ID string `db:"id"`
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
	ID   string `db:"id"`
	Slug string `db:"slug"`
	Name string `db:"name"`
}

type aliasRow struct {
	TopicID string `db:"topic_id"`
	Alias   string `db:"alias"`
}

type postRow struct {
	ID                string    `db:"id"`
	Title             string    `db:"title"`
	Caption           string    `db:"caption"`
	ImageURL          string    `db:"image_url"`
	ExamName          *string   `db:"exam_name"`
	CreatedAt         time.Time `db:"created_at"`
	AuthorID          string    `db:"author_id"`
	AuthorUsername    string    `db:"author_username"`
	AuthorDisplayName string    `db:"author_display_name"`
	AuthorRole        string    `db:"author_role"`
	AuthorAvatarURL   *string   `db:"author_avatar_url"`
}

type postOwnerRow struct {
	UserID string `db:"user_id"`
}

type postTopicRow struct {
	PostID string `db:"post_id"`
	ID     string `db:"id"`
	Slug   string `db:"slug"`
	Name   string `db:"name"`
}

type reactionCountRow struct {
	PostID string `db:"post_id"`
	Total  int64  `db:"total"`
}

type commentCountRow struct {
	PostID string `db:"post_id"`
	Total  int64  `db:"total"`
}

func hydrateTopics(source db.Tx, rows []topicRow) ([]Topic, error) {
	topics := make([]Topic, 0, len(rows))
	if len(rows) == 0 {
		return topics, nil
	}

	topicIDs := make([]string, 0, len(rows))
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

func loadAliases(source db.Tx, topicIDs []string) (map[string][]string, error) {
	aliasesByTopic := make(map[string][]string, len(topicIDs))
	for _, topicID := range topicIDs {
		aliasesByTopic[topicID] = []string{}
	}
	if len(topicIDs) == 0 {
		return aliasesByTopic, nil
	}

	rows := make([]aliasRow, 0)
	err := source.Table("topic_aliases").
		Select("topic_id", "alias").
		WhereIn("topic_id", stringValues(topicIDs)).
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
		Join("post_media AS media ON media.post_id = posts.id AND media.position = 0").
		Select(
			"posts.id AS id",
			"posts.title AS title",
			"posts.caption AS caption",
			"COALESCE(media.media_url, '') AS image_url",
			"posts.exam_name AS exam_name",
			"posts.created_at AS created_at",
			"users.id AS author_id",
			"users.username AS author_username",
			"users.display_name AS author_display_name",
			"users.role AS author_role",
			"users.avatar_url AS author_avatar_url",
		)
}

func getPost(source db.Tx, viewerID, postID string) (Post, error) {
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

func hydratePosts(source db.Tx, rows []postRow, viewerID string) ([]Post, error) {
	posts := make([]Post, 0, len(rows))
	if len(rows) == 0 {
		return posts, nil
	}

	postIDs := make([]string, 0, len(rows))
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
	commentCounts, err := loadCommentCounts(source, postIDs)
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
			CommentCount:     commentCounts[row.ID],
			ViewerHasReacted: hasReacted,
		})
	}

	return posts, nil
}

func loadPostTopics(source db.Tx, postIDs []string) (map[string][]Topic, error) {
	topicsByPost := make(map[string][]Topic, len(postIDs))
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
		WhereIn("post_topics.post_id", stringValues(postIDs)).
		OrderBy("post_topics.post_id").
		OrderBy("topics.id").
		Get(&rows)
	if err != nil {
		return nil, err
	}

	topicIDs := make([]string, 0, len(rows))
	seenTopics := make(map[string]struct{}, len(rows))
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

func loadReactionCounts(source db.Tx, postIDs []string) (map[string]int64, error) {
	counts := make(map[string]int64, len(postIDs))
	rows := make([]reactionCountRow, 0)

	err := source.Table("post_reactions").
		Select("post_id", "COUNT(*) AS total").
		WhereIn("post_id", stringValues(postIDs)).
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

func loadCommentCounts(source db.Tx, postIDs []string) (map[string]int64, error) {
	counts := make(map[string]int64, len(postIDs))
	rows := make([]commentCountRow, 0)

	err := source.Table("comments").
		Select("post_id", "COUNT(*) AS total").
		WhereIn("post_id", stringValues(postIDs)).
		Where("status", models.CommentStatusVisible).
		WhereNull("deleted_at").
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
	postIDs []string,
	viewerID string,
) (map[string]struct{}, error) {
	var reactedPostIDs []string
	err := source.Table("post_reactions").
		Where("user_id", viewerID).
		WhereIn("post_id", stringValues(postIDs)).
		Pluck("post_id", &reactedPostIDs)
	if err != nil {
		return nil, err
	}

	reactions := make(map[string]struct{}, len(reactedPostIDs))
	for _, postID := range reactedPostIDs {
		reactions[postID] = struct{}{}
	}

	return reactions, nil
}

func publishedPostExists(source db.Tx, postID string) (bool, error) {
	return source.Table("posts").
		Where("id", postID).
		Where("status", models.PostStatusPublished).
		Exists()
}

func publishedPostExistsForUpdate(source db.Tx, postID string) (bool, error) {
	rows := make([]lockedResourceRow, 0, 1)
	err := source.Table("posts").
		Select("id").
		Where("id", postID).
		Where("status", models.PostStatusPublished).
		LockForUpdate().
		Limit(1).
		Get(&rows)
	if err != nil {
		return false, err
	}

	return len(rows) > 0, nil
}

func putReactionIdempotently(source db.Tx, userID, postID string) error {
	identity := map[string]any{
		"post_id": postID,
		"user_id": userID,
	}
	values := map[string]any{
		"post_id":          postID,
		"user_id":          userID,
		"reaction_type_id": defaultLikeReactionTypeID,
	}

	var mutationErr error
	for range 2 {
		_, mutationErr = source.Table("post_reactions").UpdateOrInsert(identity, values)

		rows := make([]lockedResourceRow, 0, 1)
		existsErr := source.Table("post_reactions").
			Select("id").
			Where(identity).
			LockForUpdate().
			Limit(1).
			Get(&rows)
		if existsErr != nil {
			if mutationErr != nil {
				return errors.Join(mutationErr, existsErr)
			}
			return existsErr
		}
		if len(rows) > 0 {
			return nil
		}
	}

	if mutationErr != nil {
		return mutationErr
	}
	return errReactionStateNotPersisted
}

func stringValues(values []string) []any {
	result := make([]any, len(values))
	for index, value := range values {
		result[index] = value
	}

	return result
}

func pageOffset(page, pageSize int) uint64 {
	return uint64((page - 1) * pageSize)
}
