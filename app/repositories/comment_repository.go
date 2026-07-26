package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	contractsdb "github.com/goravel/framework/contracts/database/db"

	"goravel/app/facades"
)

const findViewablePublishedCommentPostSQL = `SELECT
	p.id,
	p.user_id AS author_id,
	p.comment_policy
FROM posts p
WHERE p.id = $1
  AND p.status = 'PUBLISHED'
  AND p.deleted_at IS NULL
  AND (
	p.user_id = $2
	OR (
		NOT EXISTS (
			SELECT 1
			FROM user_blocks b
			WHERE (b.blocker_id = p.user_id AND b.blocked_id = $2)
			   OR (b.blocker_id = $2 AND b.blocked_id = p.user_id)
		)
		AND (
			p.visibility = 'PUBLIC'
			OR (
				p.visibility = 'FOLLOWERS'
				AND EXISTS (
					SELECT 1
					FROM follows f
					WHERE f.follower_id = $2
					  AND f.following_id = p.user_id
					  AND f.status = 'ACCEPTED'
				)
			)
		)
	)
  )
LIMIT 1`

const findViewablePublishedCommentPostForShareSQL = `SELECT
	p.id,
	p.user_id AS author_id,
	p.comment_policy
FROM posts p
WHERE p.id = $1
  AND p.status = 'PUBLISHED'
  AND p.deleted_at IS NULL
  AND (
	p.user_id = $2
	OR (
		NOT EXISTS (
			SELECT 1
			FROM user_blocks b
			WHERE (b.blocker_id = p.user_id AND b.blocked_id = $2)
			   OR (b.blocker_id = $2 AND b.blocked_id = p.user_id)
		)
		AND (
			p.visibility = 'PUBLIC'
			OR (
				p.visibility = 'FOLLOWERS'
				AND EXISTS (
					SELECT 1
					FROM follows f
					WHERE f.follower_id = $2
					  AND f.following_id = p.user_id
					  AND f.status = 'ACCEPTED'
				)
			)
		)
	)
  )
LIMIT 1
FOR SHARE`

const countVisibleCommentsSQL = `SELECT COUNT(*) AS total
FROM comments c
WHERE c.post_id = $1
  AND c.status = 'VISIBLE'
  AND c.deleted_at IS NULL
  AND EXISTS (
	SELECT 1
	FROM posts p
	WHERE p.id = c.post_id
	  AND p.status = 'PUBLISHED'
	  AND p.deleted_at IS NULL
	  AND (
		p.user_id = $2
		OR (
			NOT EXISTS (
				SELECT 1
				FROM user_blocks b
				WHERE (b.blocker_id = p.user_id AND b.blocked_id = $2)
				   OR (b.blocker_id = $2 AND b.blocked_id = p.user_id)
			)
			AND (
				p.visibility = 'PUBLIC'
				OR (
					p.visibility = 'FOLLOWERS'
					AND EXISTS (
						SELECT 1
						FROM follows f
						WHERE f.follower_id = $2
						  AND f.following_id = p.user_id
						  AND f.status = 'ACCEPTED'
					)
				)
			)
		)
	  )
  )`

const listVisibleCommentsSQL = `SELECT
	c.id,
	c.post_id,
	c.body,
	c.created_at,
	u.id AS author_id,
	u.username AS author_username,
	u.display_name AS author_display_name,
	u.role AS author_role,
	u.avatar_url AS author_avatar_url
FROM comments c
JOIN users u ON u.id = c.user_id
WHERE c.post_id = $1
  AND c.status = 'VISIBLE'
  AND c.deleted_at IS NULL
  AND EXISTS (
	SELECT 1
	FROM posts p
	WHERE p.id = c.post_id
	  AND p.status = 'PUBLISHED'
	  AND p.deleted_at IS NULL
	  AND (
		p.user_id = $2
		OR (
			NOT EXISTS (
				SELECT 1
				FROM user_blocks b
				WHERE (b.blocker_id = p.user_id AND b.blocked_id = $2)
				   OR (b.blocker_id = $2 AND b.blocked_id = p.user_id)
			)
			AND (
				p.visibility = 'PUBLIC'
				OR (
					p.visibility = 'FOLLOWERS'
					AND EXISTS (
						SELECT 1
						FROM follows f
						WHERE f.follower_id = $2
						  AND f.following_id = p.user_id
						  AND f.status = 'ACCEPTED'
					)
				)
			)
		)
	  )
  )
ORDER BY c.created_at DESC, c.id DESC
LIMIT $3 OFFSET $4`

const insertVisibleCommentSQL = `WITH allowed_post AS (
	SELECT p.id
	FROM posts p
	WHERE p.id = $1
	  AND p.status = 'PUBLISHED'
	  AND p.deleted_at IS NULL
	  AND (
		p.user_id = $2
		OR (
			NOT EXISTS (
				SELECT 1
				FROM user_blocks b
				WHERE (b.blocker_id = p.user_id AND b.blocked_id = $2)
				   OR (b.blocker_id = $2 AND b.blocked_id = p.user_id)
			)
			AND (
				p.visibility = 'PUBLIC'
				OR (
					p.visibility = 'FOLLOWERS'
					AND EXISTS (
						SELECT 1
						FROM follows view_follow
						WHERE view_follow.follower_id = $2
						  AND view_follow.following_id = p.user_id
						  AND view_follow.status = 'ACCEPTED'
					)
				)
			)
		)
	  )
	  AND p.comment_policy <> 'NONE'
	  AND (
		p.user_id = $2
		OR p.comment_policy = 'EVERYONE'
		OR (
			p.comment_policy = 'FOLLOWERS'
			AND EXISTS (
				SELECT 1
				FROM follows f
				WHERE f.follower_id = $2
				  AND f.following_id = p.user_id
				  AND f.status = 'ACCEPTED'
			)
		)
	  )
),
inserted AS (
	INSERT INTO comments (
		post_id,
		user_id,
		body,
		status
	)
	SELECT allowed_post.id, $2, $3, 'VISIBLE'
	FROM allowed_post
	RETURNING id, post_id, user_id, body, created_at
)
SELECT
	inserted.id,
	inserted.post_id,
	inserted.body,
	inserted.created_at,
	u.id AS author_id,
	u.username AS author_username,
	u.display_name AS author_display_name,
	u.role AS author_role,
	u.avatar_url AS author_avatar_url
FROM inserted
JOIN users u ON u.id = inserted.user_id`

const softDeleteOwnedVisibleCommentSQL = `UPDATE comments
SET
	status = 'REMOVED',
	deleted_at = CURRENT_TIMESTAMP,
	updated_at = CURRENT_TIMESTAMP
WHERE id = $1
  AND post_id = $2
  AND user_id = $3
  AND status = 'VISIBLE'
  AND deleted_at IS NULL`

type Comment struct {
	ID        string    `json:"id"`
	PostID    string    `json:"postId"`
	Body      string    `json:"body"`
	Author    User      `json:"author"`
	CreatedAt time.Time `json:"createdAt"`
}

type publishedCommentPostRow struct {
	ID            string `db:"id"`
	AuthorID      string `db:"author_id"`
	CommentPolicy string `db:"comment_policy"`
}

type commentTotalRow struct {
	Total int64 `db:"total"`
}

type commentRow struct {
	ID                string    `db:"id"`
	PostID            string    `db:"post_id"`
	Body              string    `db:"body"`
	CreatedAt         time.Time `db:"created_at"`
	AuthorID          string    `db:"author_id"`
	AuthorUsername    string    `db:"author_username"`
	AuthorDisplayName string    `db:"author_display_name"`
	AuthorRole        string    `db:"author_role"`
	AuthorAvatarURL   *string   `db:"author_avatar_url"`
}

type GoravelCommentRepository struct {
	database contractsdb.DB
}

func NewCommentRepository(database ...contractsdb.DB) *GoravelCommentRepository {
	if len(database) > 0 && database[0] != nil {
		return &GoravelCommentRepository{database: database[0]}
	}

	return &GoravelCommentRepository{database: facades.DB()}
}

func (r *GoravelCommentRepository) UserExists(
	ctx context.Context,
	userID string,
) (bool, error) {
	return r.database.WithContext(ctx).
		Table("users").
		Where("id", userID).
		Exists()
}

func (r *GoravelCommentRepository) ListByPost(
	ctx context.Context,
	viewerID string,
	postID string,
	page int,
	pageSize int,
) ([]Comment, int64, error) {
	database := r.database.WithContext(ctx)

	_, exists, err := findViewablePublishedCommentPost(
		database,
		postID,
		viewerID,
		false,
	)
	if err != nil {
		return nil, 0, err
	}
	if !exists {
		return nil, 0, ErrNotFound
	}

	totalRows := make([]commentTotalRow, 0, 1)
	if err = database.Select(
		&totalRows,
		countVisibleCommentsSQL,
		postID,
		viewerID,
	); err != nil {
		return nil, 0, err
	}
	if len(totalRows) != 1 {
		return nil, 0, fmt.Errorf("không đọc được tổng số bình luận")
	}

	rows := make([]commentRow, 0)
	if err = database.Select(
		&rows,
		listVisibleCommentsSQL,
		postID,
		viewerID,
		pageSize,
		(page-1)*pageSize,
	); err != nil {
		return nil, 0, err
	}

	return mapComments(rows), totalRows[0].Total, nil
}

func (r *GoravelCommentRepository) Create(
	ctx context.Context,
	userID string,
	postID string,
	body string,
) (Comment, error) {
	var created commentRow

	err := r.database.WithContext(ctx).Transaction(func(tx contractsdb.Tx) error {
		post, exists, lookupErr := findViewablePublishedCommentPost(
			tx,
			postID,
			userID,
			true,
		)
		if lookupErr != nil {
			return lookupErr
		}
		if !exists {
			return ErrNotFound
		}
		if post.CommentPolicy == "NONE" {
			return ErrForbidden
		}
		if post.CommentPolicy != "EVERYONE" &&
			post.CommentPolicy != "FOLLOWERS" {
			return ErrForbidden
		}

		if insertErr := tx.Select(
			&created,
			insertVisibleCommentSQL,
			postID,
			userID,
			body,
		); insertErr != nil {
			if errors.Is(insertErr, sql.ErrNoRows) {
				return ErrForbidden
			}
			return insertErr
		}
		if created.ID == "" {
			return ErrForbidden
		}

		return nil
	})
	if err != nil {
		return Comment{}, err
	}

	return mapComment(created), nil
}

func (r *GoravelCommentRepository) Delete(
	ctx context.Context,
	userID string,
	postID string,
	commentID string,
) error {
	result, err := r.database.WithContext(ctx).Update(
		softDeleteOwnedVisibleCommentSQL,
		commentID,
		postID,
		userID,
	)
	if err != nil {
		return err
	}
	if result == nil || result.RowsAffected != 1 {
		return ErrNotFound
	}

	return nil
}

func findViewablePublishedCommentPost(
	source contractsdb.Tx,
	postID string,
	viewerID string,
	lockForShare bool,
) (publishedCommentPostRow, bool, error) {
	query := findViewablePublishedCommentPostSQL
	if lockForShare {
		query = findViewablePublishedCommentPostForShareSQL
	}

	rows := make([]publishedCommentPostRow, 0, 1)
	if err := source.Select(&rows, query, postID, viewerID); err != nil {
		return publishedCommentPostRow{}, false, err
	}
	if len(rows) == 0 {
		return publishedCommentPostRow{}, false, nil
	}

	return rows[0], true, nil
}

func mapComments(rows []commentRow) []Comment {
	comments := make([]Comment, 0, len(rows))
	for _, row := range rows {
		comments = append(comments, mapComment(row))
	}
	return comments
}

func mapComment(row commentRow) Comment {
	return Comment{
		ID:        row.ID,
		PostID:    row.PostID,
		Body:      row.Body,
		CreatedAt: row.CreatedAt,
		Author: User{
			ID:          row.AuthorID,
			Username:    row.AuthorUsername,
			DisplayName: row.AuthorDisplayName,
			Role:        row.AuthorRole,
			AvatarURL:   row.AuthorAvatarURL,
		},
	}
}
