package repositories

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	contractsdb "github.com/goravel/framework/contracts/database/db"
)

const (
	commentRepositoryViewerID = "00000000-0000-4000-8000-000000000001"
	commentRepositoryAuthorID = "00000000-0000-4000-8000-000000000002"
	commentRepositoryPostID   = "20000000-0000-4000-8000-000000000001"
	commentRepositoryID       = "50000000-0000-4000-8000-000000000001"
)

type recordedCommentSelect struct {
	query string
	args  []any
}

type recordedCommentUpdate struct {
	query string
	args  []any
}

type commentSelectStep func(destination any)

type recordingCommentDB struct {
	contractsdb.DB
	selects         []recordedCommentSelect
	selectSteps     []commentSelectStep
	selectErrAt     map[int]error
	updates         []recordedCommentUpdate
	updateResult    *contractsdb.Result
	updateErr       error
	transaction     *recordingCommentTx
	transactionErr  error
	transactionRuns int
}

func (f *recordingCommentDB) WithContext(context.Context) contractsdb.DB {
	return f
}

func (f *recordingCommentDB) Select(destination any, query string, args ...any) error {
	callIndex := len(f.selects)
	f.selects = append(f.selects, recordedCommentSelect{
		query: query,
		args:  append([]any(nil), args...),
	})
	if err := f.selectErrAt[callIndex]; err != nil {
		return err
	}
	if callIndex < len(f.selectSteps) && f.selectSteps[callIndex] != nil {
		f.selectSteps[callIndex](destination)
	}
	return nil
}

func (f *recordingCommentDB) Transaction(callback func(contractsdb.Tx) error) error {
	f.transactionRuns++
	if f.transactionErr != nil {
		return f.transactionErr
	}
	return callback(f.transaction)
}

func (f *recordingCommentDB) Update(
	query string,
	args ...any,
) (*contractsdb.Result, error) {
	f.updates = append(f.updates, recordedCommentUpdate{
		query: query,
		args:  append([]any(nil), args...),
	})
	return f.updateResult, f.updateErr
}

type recordingCommentTx struct {
	contractsdb.Tx
	selects     []recordedCommentSelect
	selectSteps []commentSelectStep
	selectErrAt map[int]error
}

func (f *recordingCommentTx) Select(destination any, query string, args ...any) error {
	callIndex := len(f.selects)
	f.selects = append(f.selects, recordedCommentSelect{
		query: query,
		args:  append([]any(nil), args...),
	})
	if err := f.selectErrAt[callIndex]; err != nil {
		return err
	}
	if callIndex < len(f.selectSteps) && f.selectSteps[callIndex] != nil {
		f.selectSteps[callIndex](destination)
	}
	return nil
}

func TestCommentRepositoryListReturnsOnlyVisibleCommentsInStableDescendingOrder(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, time.July, 25, 9, 30, 0, 0, time.UTC)
	database := &recordingCommentDB{
		selectSteps: []commentSelectStep{
			func(destination any) {
				rows := destination.(*[]publishedCommentPostRow)
				*rows = append(*rows, publishedCommentPostRow{
					ID:            commentRepositoryPostID,
					AuthorID:      commentRepositoryAuthorID,
					CommentPolicy: "EVERYONE",
				})
			},
			func(destination any) {
				rows := destination.(*[]commentTotalRow)
				*rows = append(*rows, commentTotalRow{Total: 21})
			},
			func(destination any) {
				rows := destination.(*[]commentRow)
				*rows = append(*rows, commentRow{
					ID:                commentRepositoryID,
					PostID:            commentRepositoryPostID,
					Body:              "<b>Bài vẽ rất đẹp!</b>",
					CreatedAt:         createdAt,
					AuthorID:          commentRepositoryAuthorID,
					AuthorUsername:    "co.mai",
					AuthorDisplayName: "Cô Mai",
					AuthorRole:        "TEACHER",
				})
			},
		},
	}
	repository := NewCommentRepository(database)

	comments, total, err := repository.ListByPost(
		context.Background(),
		commentRepositoryViewerID,
		commentRepositoryPostID,
		2,
		10,
	)

	if err != nil {
		t.Fatalf("ListByPost error = %v", err)
	}
	if total != 21 || len(comments) != 1 {
		t.Fatalf("ListByPost returned %d comments and total %d", len(comments), total)
	}
	if comments[0].Body != "<b>Bài vẽ rất đẹp!</b>" {
		t.Fatalf("comment body = %q, want plain text preserved", comments[0].Body)
	}
	if comments[0].CreatedAt != createdAt ||
		comments[0].Author.ID != commentRepositoryAuthorID {
		t.Fatalf("comment mapping = %#v", comments[0])
	}
	if len(database.selects) != 3 {
		t.Fatalf("raw select calls = %d, want post, count and list", len(database.selects))
	}

	accessCall := database.selects[0]
	for _, fragment := range []string{
		"p.user_id = $2",
		"FROM user_blocks b",
		"b.blocker_id = p.user_id AND b.blocked_id = $2",
		"b.blocker_id = $2 AND b.blocked_id = p.user_id",
		"p.visibility = 'PUBLIC'",
		"p.visibility = 'FOLLOWERS'",
		"f.status = 'ACCEPTED'",
	} {
		if !strings.Contains(accessCall.query, fragment) {
			t.Fatalf("view access SQL missing %q: %s", fragment, accessCall.query)
		}
	}
	if !reflect.DeepEqual(
		accessCall.args,
		[]any{commentRepositoryPostID, commentRepositoryViewerID},
	) {
		t.Fatalf("view access args = %#v", accessCall.args)
	}
	if strings.Contains(accessCall.query, commentRepositoryViewerID) ||
		strings.Contains(accessCall.query, commentRepositoryPostID) {
		t.Fatalf("viewer or post ID was concatenated into access SQL: %s", accessCall.query)
	}

	countCall := database.selects[1]
	listCall := database.selects[2]
	for callName, call := range map[string]recordedCommentSelect{
		"count": countCall,
		"list":  listCall,
	} {
		for _, fragment := range []string{
			"p.user_id = $2",
			"FROM user_blocks b",
			"p.visibility = 'PUBLIC'",
			"p.visibility = 'FOLLOWERS'",
			"f.status = 'ACCEPTED'",
		} {
			if !strings.Contains(call.query, fragment) {
				t.Fatalf("%s SQL missing view check %q: %s", callName, fragment, call.query)
			}
		}
	}
	if !reflect.DeepEqual(
		countCall.args,
		[]any{commentRepositoryPostID, commentRepositoryViewerID},
	) {
		t.Fatalf("count args = %#v, want bound post/viewer IDs", countCall.args)
	}
	for _, fragment := range []string{
		"c.status = 'VISIBLE'",
		"c.deleted_at IS NULL",
		"ORDER BY c.created_at DESC, c.id DESC",
		"LIMIT $3 OFFSET $4",
	} {
		if !strings.Contains(listCall.query, fragment) {
			t.Fatalf("list SQL missing %q: %s", fragment, listCall.query)
		}
	}
	if strings.Contains(listCall.query, commentRepositoryPostID) {
		t.Fatalf("post ID was concatenated into SQL: %s", listCall.query)
	}
	if !reflect.DeepEqual(
		listCall.args,
		[]any{commentRepositoryPostID, commentRepositoryViewerID, 10, 10},
	) {
		t.Fatalf("list args = %#v, want bound post/viewer/page values", listCall.args)
	}
}

func TestCommentRepositoryListHidesMissingOrNonViewablePost(t *testing.T) {
	t.Parallel()

	database := &recordingCommentDB{}
	repository := NewCommentRepository(database)

	comments, total, err := repository.ListByPost(
		context.Background(),
		commentRepositoryViewerID,
		commentRepositoryPostID,
		1,
		20,
	)

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("ListByPost error = %v, want ErrNotFound", err)
	}
	if comments != nil || total != 0 {
		t.Fatalf("ListByPost returned (%#v, %d), want nil/zero", comments, total)
	}
	if len(database.selects) != 1 ||
		!reflect.DeepEqual(database.selects[0].args, []any{
			commentRepositoryPostID,
			commentRepositoryViewerID,
		}) {
		t.Fatalf("viewable published post lookup = %#v", database.selects)
	}
}

func TestCommentRepositoryCreateEnforcesFollowerPolicyAndBindsPlainText(t *testing.T) {
	t.Parallel()

	body := "<strong>Em cảm ơn cô!</strong>"
	createdAt := time.Date(2026, time.July, 25, 10, 0, 0, 0, time.UTC)
	transaction := &recordingCommentTx{
		selectSteps: []commentSelectStep{
			func(destination any) {
				rows := destination.(*[]publishedCommentPostRow)
				*rows = append(*rows, publishedCommentPostRow{
					ID:            commentRepositoryPostID,
					AuthorID:      commentRepositoryAuthorID,
					CommentPolicy: "FOLLOWERS",
				})
			},
			func(destination any) {
				row := destination.(*commentRow)
				*row = commentRow{
					ID:                commentRepositoryID,
					PostID:            commentRepositoryPostID,
					Body:              body,
					CreatedAt:         createdAt,
					AuthorID:          commentRepositoryViewerID,
					AuthorUsername:    "linh.ve",
					AuthorDisplayName: "Gia Linh",
					AuthorRole:        "STUDENT",
				}
			},
		},
	}
	database := &recordingCommentDB{transaction: transaction}
	repository := NewCommentRepository(database)

	comment, err := repository.Create(
		context.Background(),
		commentRepositoryViewerID,
		commentRepositoryPostID,
		body,
	)

	if err != nil {
		t.Fatalf("Create error = %v", err)
	}
	if database.transactionRuns != 1 {
		t.Fatalf("transaction runs = %d, want 1", database.transactionRuns)
	}
	if comment.ID != commentRepositoryID || comment.Body != body {
		t.Fatalf("created comment = %#v", comment)
	}
	if len(transaction.selects) != 2 {
		t.Fatalf("transaction selects = %#v", transaction.selects)
	}
	accessCall := transaction.selects[0]
	if !reflect.DeepEqual(
		accessCall.args,
		[]any{commentRepositoryPostID, commentRepositoryViewerID},
	) {
		t.Fatalf("view access args = %#v", accessCall.args)
	}
	insertCall := transaction.selects[1]
	for _, fragment := range []string{
		"FROM user_blocks b",
		"p.visibility = 'PUBLIC'",
		"p.visibility = 'FOLLOWERS'",
		"p.comment_policy <> 'NONE'",
		"p.user_id = $2",
		"p.comment_policy = 'EVERYONE'",
		"p.comment_policy = 'FOLLOWERS'",
		"f.status = 'ACCEPTED'",
	} {
		if !strings.Contains(insertCall.query, fragment) {
			t.Fatalf("policy-aware insert SQL missing %q: %s", fragment, insertCall.query)
		}
	}
	if strings.Contains(insertCall.query, body) ||
		strings.Contains(insertCall.query, commentRepositoryViewerID) {
		t.Fatalf("user input was concatenated into insert SQL: %s", insertCall.query)
	}
	if !reflect.DeepEqual(
		insertCall.args,
		[]any{commentRepositoryPostID, commentRepositoryViewerID, body},
	) {
		t.Fatalf("insert args = %#v, want bound post/author/body", insertCall.args)
	}
}

func TestCommentRepositoryCreateDistinguishesMissingPostAndForbiddenPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		selectSteps []commentSelectStep
		wantErr     error
		wantCalls   int
	}{
		{
			name:        "post is not published",
			selectSteps: []commentSelectStep{nil},
			wantErr:     ErrNotFound,
			wantCalls:   1,
		},
		{
			name: "comments disabled",
			selectSteps: []commentSelectStep{
				func(destination any) {
					rows := destination.(*[]publishedCommentPostRow)
					*rows = append(*rows, publishedCommentPostRow{
						ID:            commentRepositoryPostID,
						AuthorID:      commentRepositoryAuthorID,
						CommentPolicy: "NONE",
					})
				},
			},
			wantErr:   ErrForbidden,
			wantCalls: 1,
		},
		{
			name: "follower required",
			selectSteps: []commentSelectStep{
				func(destination any) {
					rows := destination.(*[]publishedCommentPostRow)
					*rows = append(*rows, publishedCommentPostRow{
						ID:            commentRepositoryPostID,
						AuthorID:      commentRepositoryAuthorID,
						CommentPolicy: "FOLLOWERS",
					})
				},
				nil,
			},
			wantErr:   ErrForbidden,
			wantCalls: 2,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			transaction := &recordingCommentTx{selectSteps: tt.selectSteps}
			repository := NewCommentRepository(
				&recordingCommentDB{transaction: transaction},
			)

			comment, err := repository.Create(
				context.Background(),
				commentRepositoryViewerID,
				commentRepositoryPostID,
				"Nội dung",
			)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Create error = %v, want %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(comment, Comment{}) {
				t.Fatalf("Create comment = %#v, want zero value", comment)
			}
			if len(transaction.selects) != tt.wantCalls {
				t.Fatalf(
					"transaction select calls = %d, want %d",
					len(transaction.selects),
					tt.wantCalls,
				)
			}
		})
	}
}

func TestCommentRepositoryCreateMapsPolicyInsertNoRowsToForbidden(t *testing.T) {
	t.Parallel()

	transaction := &recordingCommentTx{
		selectSteps: []commentSelectStep{
			func(destination any) {
				rows := destination.(*[]publishedCommentPostRow)
				*rows = append(*rows, publishedCommentPostRow{
					ID:            commentRepositoryPostID,
					AuthorID:      commentRepositoryAuthorID,
					CommentPolicy: "FOLLOWERS",
				})
			},
		},
		selectErrAt: map[int]error{1: sql.ErrNoRows},
	}
	repository := NewCommentRepository(
		&recordingCommentDB{transaction: transaction},
	)

	comment, err := repository.Create(
		context.Background(),
		commentRepositoryViewerID,
		commentRepositoryPostID,
		"Nội dung hợp lệ",
	)

	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("Create error = %v, want ErrForbidden", err)
	}
	if !reflect.DeepEqual(comment, Comment{}) {
		t.Fatalf("Create comment = %#v, want zero value", comment)
	}
}

func TestCommentRepositoryDeleteSoftDeletesOnlyVisibleOwnedCommentInPost(t *testing.T) {
	t.Parallel()

	database := &recordingCommentDB{
		updateResult: &contractsdb.Result{RowsAffected: 1},
	}
	repository := NewCommentRepository(database)

	err := repository.Delete(
		context.Background(),
		commentRepositoryViewerID,
		commentRepositoryPostID,
		commentRepositoryID,
	)

	if err != nil {
		t.Fatalf("Delete error = %v", err)
	}
	if len(database.updates) != 1 {
		t.Fatalf("update calls = %#v, want one", database.updates)
	}

	update := database.updates[0]
	for _, fragment := range []string{
		"UPDATE comments",
		"status = 'REMOVED'",
		"deleted_at = CURRENT_TIMESTAMP",
		"updated_at = CURRENT_TIMESTAMP",
		"id = $1",
		"post_id = $2",
		"user_id = $3",
		"status = 'VISIBLE'",
		"deleted_at IS NULL",
	} {
		if !strings.Contains(update.query, fragment) {
			t.Fatalf("delete SQL missing %q: %s", fragment, update.query)
		}
	}
	if strings.Contains(update.query, commentRepositoryViewerID) ||
		strings.Contains(update.query, commentRepositoryPostID) ||
		strings.Contains(update.query, commentRepositoryID) {
		t.Fatalf("comment delete input was concatenated into SQL: %s", update.query)
	}
	if !reflect.DeepEqual(update.args, []any{
		commentRepositoryID,
		commentRepositoryPostID,
		commentRepositoryViewerID,
	}) {
		t.Fatalf("delete args = %#v", update.args)
	}
}

func TestCommentRepositoryDeleteHidesMissingOrUnownedTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		result *contractsdb.Result
	}{
		{name: "no matching row", result: &contractsdb.Result{}},
		{name: "nil result", result: nil},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repository := NewCommentRepository(
				&recordingCommentDB{updateResult: tt.result},
			)

			err := repository.Delete(
				context.Background(),
				commentRepositoryViewerID,
				commentRepositoryPostID,
				commentRepositoryID,
			)

			if !errors.Is(err, ErrNotFound) {
				t.Fatalf("Delete error = %v, want ErrNotFound", err)
			}
		})
	}
}
