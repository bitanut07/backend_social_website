package repositories

import (
	"context"
	"errors"
	"reflect"
	"testing"

	contractsdb "github.com/goravel/framework/contracts/database/db"

	"goravel/app/models"
)

const (
	contentRepositoryUserID  = "00000000-0000-4000-8000-000000000001"
	contentRepositoryTopicID = "10000000-0000-4000-8000-000000000001"
	contentRepositoryPostID  = "20000000-0000-4000-8000-000000000001"
	contentRepositoryNewPost = "20000000-0000-4000-8000-000000000002"
)

type recordingContentDB struct {
	contractsdb.DB
	query            contractsdb.Query
	transaction      contractsdb.Tx
	transactionErr   error
	transactionCalls int
}

func (f *recordingContentDB) WithContext(context.Context) contractsdb.DB {
	return f
}

func (f *recordingContentDB) Table(string) contractsdb.Query {
	return f.query
}

func (f *recordingContentDB) Transaction(callback func(contractsdb.Tx) error) error {
	f.transactionCalls++
	if f.transactionErr != nil {
		return f.transactionErr
	}

	return callback(f.transaction)
}

type recordedWhere struct {
	query any
	args  []any
}

type recordedJoin struct {
	query string
	args  []any
}

type recordingContentQuery struct {
	contractsdb.Query
	joins       []recordedJoin
	wheres      []recordedWhere
	orderByDesc []string
	paginateErr error
	updateErr   error
	exists      bool
	existsErr   error
	lockCalls   int
}

func (f *recordingContentQuery) Join(query string, args ...any) contractsdb.Query {
	f.joins = append(f.joins, recordedJoin{query: query, args: append([]any(nil), args...)})
	return f
}

func (f *recordingContentQuery) Select(...string) contractsdb.Query {
	return f
}

func (f *recordingContentQuery) Where(query any, args ...any) contractsdb.Query {
	f.wheres = append(f.wheres, recordedWhere{query: query, args: append([]any(nil), args...)})
	return f
}

func (f *recordingContentQuery) OrderByDesc(column string) contractsdb.Query {
	f.orderByDesc = append(f.orderByDesc, column)
	return f
}

func (f *recordingContentQuery) Paginate(int, int, any, *int64) error {
	return f.paginateErr
}

func (f *recordingContentQuery) UpdateOrInsert(any, any) (*contractsdb.Result, error) {
	return nil, f.updateErr
}

func (f *recordingContentQuery) LockForUpdate() contractsdb.Query {
	f.lockCalls++
	return f
}

func (f *recordingContentQuery) Exists() (bool, error) {
	return f.exists, f.existsErr
}

type recordingContentTx struct {
	contractsdb.Tx
	query          contractsdb.Query
	selectErr      error
	selectedPostID string
	selects        []recordedRawSelect
}

func (f *recordingContentTx) Table(string) contractsdb.Query {
	return f.query
}

type recordedRawSelect struct {
	query string
	args  []any
}

func (f *recordingContentTx) Select(dest any, query string, args ...any) error {
	f.selects = append(f.selects, recordedRawSelect{
		query: query,
		args:  append([]any(nil), args...),
	})
	if f.selectErr != nil {
		return f.selectErr
	}

	dest.(*insertedPostIDRow).ID = f.selectedPostID
	return nil
}

func TestContentRepositoryListPostsUsesStableOrderAndBoundTopicFilter(t *testing.T) {
	t.Parallel()

	stopAfterQuery := errors.New("stop after query construction")
	query := &recordingContentQuery{paginateErr: stopAfterQuery}
	database := &recordingContentDB{query: query}
	repository := NewContentRepository(database)
	topicID := contentRepositoryTopicID

	_, _, err := repository.ListPosts(
		context.Background(),
		contentRepositoryUserID,
		2,
		10,
		&topicID,
	)

	if !errors.Is(err, stopAfterQuery) {
		t.Fatalf("ListPosts error = %v, want query sentinel", err)
	}
	if !reflect.DeepEqual(query.orderByDesc, []string{"posts.created_at", "posts.id"}) {
		t.Fatalf("descending order = %#v, want created_at then id", query.orderByDesc)
	}
	if len(query.joins) != 2 {
		t.Fatalf("joins = %#v, want author and selected topic joins", query.joins)
	}
	if query.joins[1].query != "post_topics AS selected_topic ON selected_topic.post_id = posts.id" {
		t.Fatalf("topic join = %q", query.joins[1].query)
	}
	if len(query.wheres) != 2 {
		t.Fatalf("wheres = %#v, want status and topic filters", query.wheres)
	}
	if query.wheres[0].query != "posts.status" ||
		!reflect.DeepEqual(query.wheres[0].args, []any{models.PostStatusPublished}) {
		t.Fatalf("status filter = %#v", query.wheres[0])
	}
	if query.wheres[1].query != "selected_topic.topic_id" ||
		!reflect.DeepEqual(query.wheres[1].args, []any{topicID}) {
		t.Fatalf("topic filter must keep ID as a separate bound value: %#v", query.wheres[1])
	}
}

func TestContentRepositoryMutationsStartTransactions(t *testing.T) {
	t.Parallel()

	transactionFailure := errors.New("transaction unavailable")
	tests := []struct {
		name string
		call func(*GoravelContentRepository) error
	}{
		{
			name: "create post",
			call: func(repository *GoravelContentRepository) error {
				_, err := repository.CreatePost(context.Background(), contentRepositoryUserID, CreatePostInput{
					Title:    "Bình minh",
					Caption:  "Màu nước.",
					ImageURL: "https://example.com/art.jpg",
					TopicIDs: []string{contentRepositoryTopicID},
				})
				return err
			},
		},
		{
			name: "put reaction",
			call: func(repository *GoravelContentRepository) error {
				_, err := repository.PutReaction(
					context.Background(),
					contentRepositoryUserID,
					contentRepositoryPostID,
				)
				return err
			},
		},
		{
			name: "delete reaction",
			call: func(repository *GoravelContentRepository) error {
				_, err := repository.DeleteReaction(
					context.Background(),
					contentRepositoryUserID,
					contentRepositoryPostID,
				)
				return err
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			database := &recordingContentDB{transactionErr: transactionFailure}
			repository := NewContentRepository(database)

			err := tt.call(repository)

			if !errors.Is(err, transactionFailure) {
				t.Fatalf("mutation error = %v, want transaction failure", err)
			}
			if database.transactionCalls != 1 {
				t.Fatalf("transaction calls = %d, want 1", database.transactionCalls)
			}
		})
	}
}

func TestInsertPostReturningIDUsesPostgreSQLReturningAndBoundValues(t *testing.T) {
	t.Parallel()

	examName := "Thi vẽ Mùa hè"
	input := CreatePostInput{
		Title:    "Sắc màu tuổi thơ",
		Caption:  "Bài vẽ màu nước.",
		ImageURL: "https://example.com/art.jpg",
		ExamName: &examName,
	}
	transaction := &recordingContentTx{selectedPostID: contentRepositoryNewPost}

	postID, err := insertPostReturningID(transaction, contentRepositoryUserID, input)

	if err != nil {
		t.Fatalf("insertPostReturningID error = %v", err)
	}
	if postID != contentRepositoryNewPost {
		t.Fatalf(
			"insertPostReturningID post ID = %q, want %q",
			postID,
			contentRepositoryNewPost,
		)
	}
	if len(transaction.selects) != 1 {
		t.Fatalf("raw select calls = %d, want 1", len(transaction.selects))
	}

	wantQuery := `INSERT INTO posts (
	user_id,
	title,
	caption,
	image_url,
	exam_name,
	status
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id`
	if transaction.selects[0].query != wantQuery {
		t.Fatalf("insert query = %q, want %q", transaction.selects[0].query, wantQuery)
	}

	wantArgs := []any{
		contentRepositoryUserID,
		input.Title,
		input.Caption,
		input.ImageURL,
		input.ExamName,
		models.PostStatusPublished,
	}
	if !reflect.DeepEqual(transaction.selects[0].args, wantArgs) {
		t.Fatalf("insert args = %#v, want %#v", transaction.selects[0].args, wantArgs)
	}
}

func TestCreatePostReturnsPostgreSQLInsertError(t *testing.T) {
	t.Parallel()

	insertFailure := errors.New("insert returning failed")
	transaction := &recordingContentTx{selectErr: insertFailure}
	database := &recordingContentDB{transaction: transaction}
	repository := NewContentRepository(database)
	input := CreatePostInput{
		Title:    "Bình minh",
		Caption:  "Màu nước.",
		ImageURL: "https://example.com/art.jpg",
	}

	post, err := repository.CreatePost(context.Background(), contentRepositoryUserID, input)

	if !errors.Is(err, insertFailure) {
		t.Fatalf("CreatePost error = %v, want insert failure", err)
	}
	if !reflect.DeepEqual(post, Post{}) {
		t.Fatalf("CreatePost post = %#v, want zero value", post)
	}
	if database.transactionCalls != 1 {
		t.Fatalf("transaction calls = %d, want 1", database.transactionCalls)
	}
	if len(transaction.selects) != 1 {
		t.Fatalf("raw select calls = %d, want 1", len(transaction.selects))
	}
	if !reflect.DeepEqual(transaction.selects[0].args, []any{
		contentRepositoryUserID,
		input.Title,
		input.Caption,
		input.ImageURL,
		input.ExamName,
		models.PostStatusPublished,
	}) {
		t.Fatalf("insert args = %#v, want bound create-post values", transaction.selects[0].args)
	}
}

func TestPutReactionIdempotentlyAcceptsAConcurrentWinner(t *testing.T) {
	t.Parallel()

	duplicateInsert := errors.New("duplicate key")
	query := &recordingContentQuery{
		updateErr: duplicateInsert,
		exists:    true,
	}
	transaction := &recordingContentTx{query: query}

	err := putReactionIdempotently(
		transaction,
		contentRepositoryUserID,
		contentRepositoryPostID,
	)

	if err != nil {
		t.Fatalf("putReactionIdempotently error = %v, want success because reaction exists", err)
	}
	if query.lockCalls != 1 {
		t.Fatalf("locking existence checks = %d, want 1", query.lockCalls)
	}
	if len(query.wheres) != 1 ||
		!reflect.DeepEqual(query.wheres[0].query, map[string]any{
			"post_id": contentRepositoryPostID,
			"user_id": contentRepositoryUserID,
		}) {
		t.Fatalf("retry lookup = %#v, want bound post/user identity", query.wheres)
	}
}

func TestPublishedPostExistsForUpdateLocksParentRow(t *testing.T) {
	t.Parallel()

	query := &recordingContentQuery{exists: true}
	transaction := &recordingContentTx{query: query}

	exists, err := publishedPostExistsForUpdate(transaction, contentRepositoryPostID)

	if err != nil {
		t.Fatalf("publishedPostExistsForUpdate error = %v", err)
	}
	if !exists {
		t.Fatal("publishedPostExistsForUpdate exists = false, want true")
	}
	if query.lockCalls != 1 {
		t.Fatalf("parent post lock calls = %d, want 1", query.lockCalls)
	}
	if len(query.wheres) != 2 {
		t.Fatalf("post lock wheres = %#v, want id and status filters", query.wheres)
	}
}

func TestEmptyContentCollectionsRemainJSONArrays(t *testing.T) {
	t.Parallel()

	topics, err := hydrateTopics(nil, nil)
	if err != nil {
		t.Fatalf("hydrateTopics error: %v", err)
	}
	if topics == nil || len(topics) != 0 {
		t.Fatalf("topics = %#v, want non-nil empty slice", topics)
	}

	posts, err := hydratePosts(nil, nil, contentRepositoryUserID)
	if err != nil {
		t.Fatalf("hydratePosts error: %v", err)
	}
	if posts == nil || len(posts) != 0 {
		t.Fatalf("posts = %#v, want non-nil empty slice", posts)
	}
}

func TestStringValuesKeepsUUIDsAsBoundValues(t *testing.T) {
	t.Parallel()

	values := []string{contentRepositoryTopicID, contentRepositoryPostID}

	if got := stringValues(values); !reflect.DeepEqual(got, []any{
		contentRepositoryTopicID,
		contentRepositoryPostID,
	}) {
		t.Fatalf("stringValues() = %#v, want UUID strings as bound values", got)
	}
}
