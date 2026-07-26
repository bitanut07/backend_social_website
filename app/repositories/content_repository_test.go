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
	contentRepositoryUserID     = "00000000-0000-4000-8000-000000000001"
	contentRepositoryTopicID    = "10000000-0000-4000-8000-000000000001"
	contentRepositoryPostID     = "20000000-0000-4000-8000-000000000001"
	contentRepositoryNewPost    = "20000000-0000-4000-8000-000000000002"
	contentRepositoryReactionID = "40000000-0000-4000-8000-000000000001"
)

type recordingContentDB struct {
	contractsdb.DB
	query            contractsdb.Query
	queries          []contractsdb.Query
	tableCalls       int
	transaction      contractsdb.Tx
	transactionErr   error
	transactionCalls int
}

func (f *recordingContentDB) WithContext(context.Context) contractsdb.DB {
	return f
}

func (f *recordingContentDB) Table(string) contractsdb.Query {
	if f.tableCalls < len(f.queries) {
		query := f.queries[f.tableCalls]
		f.tableCalls++
		return query
	}
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
	joins        []recordedJoin
	selects      []string
	wheres       []recordedWhere
	whereIns     []recordedWhere
	whereNulls   []string
	orderBy      []string
	orderByDesc  []string
	groupBy      []string
	count        int64
	countErr     error
	countCalls   int
	getErr       error
	getID        string
	getUserID    string
	getRows      func(any)
	offset       uint64
	limit        uint64
	updateErr    error
	updateResult *contractsdb.Result
	updates      []recordedUpdate
	exists       bool
	existsErr    error
	lockCalls    int
}

func (f *recordingContentQuery) Join(query string, args ...any) contractsdb.Query {
	f.joins = append(f.joins, recordedJoin{query: query, args: append([]any(nil), args...)})
	return f
}

func (f *recordingContentQuery) Select(columns ...string) contractsdb.Query {
	f.selects = append(f.selects, columns...)
	return f
}

func (f *recordingContentQuery) Where(query any, args ...any) contractsdb.Query {
	f.wheres = append(f.wheres, recordedWhere{query: query, args: append([]any(nil), args...)})
	return f
}

func (f *recordingContentQuery) WhereIn(column string, values []any) contractsdb.Query {
	f.whereIns = append(f.whereIns, recordedWhere{
		query: column,
		args:  append([]any(nil), values...),
	})
	return f
}

func (f *recordingContentQuery) WhereNull(column string) contractsdb.Query {
	f.whereNulls = append(f.whereNulls, column)
	return f
}

func (f *recordingContentQuery) OrderBy(column string, _ ...string) contractsdb.Query {
	f.orderBy = append(f.orderBy, column)
	return f
}

func (f *recordingContentQuery) OrderByDesc(column string) contractsdb.Query {
	f.orderByDesc = append(f.orderByDesc, column)
	return f
}

func (f *recordingContentQuery) GroupBy(columns ...string) contractsdb.Query {
	f.groupBy = append(f.groupBy, columns...)
	return f
}

func (f *recordingContentQuery) Count() (int64, error) {
	f.countCalls++
	return f.count, f.countErr
}

func (f *recordingContentQuery) Offset(offset uint64) contractsdb.Query {
	f.offset = offset
	return f
}

func (f *recordingContentQuery) Limit(limit uint64) contractsdb.Query {
	f.limit = limit
	return f
}

func (f *recordingContentQuery) Get(destination any) error {
	if f.getRows != nil {
		f.getRows(destination)
	}
	if f.getErr == nil && (f.getID != "" || f.getUserID != "") {
		slice := reflect.ValueOf(destination).Elem()
		row := reflect.New(slice.Type().Elem()).Elem()
		if f.getID != "" {
			row.FieldByName("ID").SetString(f.getID)
		}
		if f.getUserID != "" {
			row.FieldByName("UserID").SetString(f.getUserID)
		}
		slice.Set(reflect.Append(slice, row))
	}
	return f.getErr
}

type recordedUpdate struct {
	column any
	values []any
}

func (f *recordingContentQuery) Update(
	column any,
	values ...any,
) (*contractsdb.Result, error) {
	f.updates = append(f.updates, recordedUpdate{
		column: column,
		values: append([]any(nil), values...),
	})
	if f.updateResult == nil {
		return &contractsdb.Result{RowsAffected: 1}, f.updateErr
	}
	return f.updateResult, f.updateErr
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
	queries        []contractsdb.Query
	tableCalls     int
	selectErr      error
	selectedPostID string
	selects        []recordedRawSelect
}

func (f *recordingContentTx) Table(string) contractsdb.Query {
	if f.tableCalls < len(f.queries) {
		query := f.queries[f.tableCalls]
		f.tableCalls++
		return query
	}
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
	countQuery := &recordingContentQuery{}
	query := &recordingContentQuery{getErr: stopAfterQuery}
	database := &recordingContentDB{queries: []contractsdb.Query{countQuery, query}}
	repository := NewContentRepository(database)
	topicID := contentRepositoryTopicID
	authorID := contentRepositoryUserID

	_, _, err := repository.ListPosts(
		context.Background(),
		contentRepositoryUserID,
		2,
		10,
		&topicID,
		&authorID,
	)

	if !errors.Is(err, stopAfterQuery) {
		t.Fatalf("ListPosts error = %v, want query sentinel", err)
	}
	if !reflect.DeepEqual(query.orderByDesc, []string{"posts.created_at", "posts.id"}) {
		t.Fatalf("descending order = %#v, want created_at then id", query.orderByDesc)
	}
	if len(query.joins) != 3 {
		t.Fatalf("joins = %#v, want author, media and selected topic joins", query.joins)
	}
	if len(countQuery.joins) != 2 {
		t.Fatalf("count joins = %#v, want media and selected topic joins", countQuery.joins)
	}
	if query.joins[1].query != "post_media AS media ON media.post_id = posts.id AND media.position = 0" {
		t.Fatalf("media join = %q", query.joins[1].query)
	}
	if !containsString(query.selects, "COALESCE(media.media_url, '') AS image_url") {
		t.Fatalf("selected columns = %#v, want nullable media URL normalized to an empty string", query.selects)
	}
	if query.joins[2].query != "post_topics AS selected_topic ON selected_topic.post_id = posts.id" {
		t.Fatalf("topic join = %q", query.joins[2].query)
	}
	if len(query.wheres) != 3 {
		t.Fatalf("wheres = %#v, want status, topic and author filters", query.wheres)
	}
	if query.wheres[0].query != "posts.status" ||
		!reflect.DeepEqual(query.wheres[0].args, []any{models.PostStatusPublished}) {
		t.Fatalf("status filter = %#v", query.wheres[0])
	}
	if query.wheres[1].query != "selected_topic.topic_id" ||
		!reflect.DeepEqual(query.wheres[1].args, []any{topicID}) {
		t.Fatalf("topic filter must keep ID as a separate bound value: %#v", query.wheres[1])
	}
	if query.wheres[2].query != "posts.user_id" ||
		!reflect.DeepEqual(query.wheres[2].args, []any{authorID}) {
		t.Fatalf("author filter must keep ID as a separate bound value: %#v", query.wheres[2])
	}
	if !reflect.DeepEqual(query.whereNulls, []string{"posts.deleted_at"}) {
		t.Fatalf("null filters = %#v, want soft-delete filter", query.whereNulls)
	}
	if query.offset != 10 || query.limit != 10 {
		t.Fatalf("pagination = offset %d limit %d, want 10/10", query.offset, query.limit)
	}
}

func TestContentRepositoryDeletePostOnlySoftDeletesItsAuthorPost(t *testing.T) {
	t.Parallel()

	t.Run("author", func(t *testing.T) {
		t.Parallel()

		ownerQuery := &recordingContentQuery{
			getUserID: contentRepositoryUserID,
		}
		updateQuery := &recordingContentQuery{}
		transaction := &recordingContentTx{
			queries: []contractsdb.Query{ownerQuery, updateQuery},
		}
		database := &recordingContentDB{transaction: transaction}
		repository := NewContentRepository(database)

		err := repository.DeletePost(
			context.Background(),
			contentRepositoryUserID,
			contentRepositoryPostID,
		)

		if err != nil {
			t.Fatalf("DeletePost error = %v", err)
		}
		if ownerQuery.lockCalls != 1 {
			t.Fatalf("owner lookup lock calls = %d, want 1", ownerQuery.lockCalls)
		}
		if len(updateQuery.updates) != 1 {
			t.Fatalf("post updates = %#v, want one soft-delete", updateQuery.updates)
		}
		update, ok := updateQuery.updates[0].column.(map[string]any)
		if !ok {
			t.Fatalf("soft-delete update = %#v, want map", updateQuery.updates[0].column)
		}
		if update["status"] != models.PostStatusRemoved {
			t.Fatalf("soft-delete status = %#v, want REMOVED", update["status"])
		}
		if _, ok = update["deleted_at"]; !ok {
			t.Fatalf("soft-delete update = %#v, want deleted_at", update)
		}
		if len(updateQuery.wheres) != 2 {
			t.Fatalf(
				"soft-delete wheres = %#v, want post and author IDs",
				updateQuery.wheres,
			)
		}
		if !reflect.DeepEqual(
			updateQuery.whereNulls,
			[]string{"deleted_at"},
		) {
			t.Fatalf(
				"soft-delete null filters = %#v, want deleted_at",
				updateQuery.whereNulls,
			)
		}
	})

	t.Run("different user", func(t *testing.T) {
		t.Parallel()

		ownerQuery := &recordingContentQuery{
			getUserID: contentRepositoryUserID,
		}
		updateQuery := &recordingContentQuery{}
		transaction := &recordingContentTx{
			queries: []contractsdb.Query{ownerQuery, updateQuery},
		}
		repository := NewContentRepository(
			&recordingContentDB{transaction: transaction},
		)

		err := repository.DeletePost(
			context.Background(),
			"00000000-0000-4000-8000-000000000099",
			contentRepositoryPostID,
		)

		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("DeletePost error = %v, want ErrForbidden", err)
		}
		if len(updateQuery.updates) != 0 {
			t.Fatalf(
				"non-author must not update post: %#v",
				updateQuery.updates,
			)
		}
	})

	t.Run("missing post", func(t *testing.T) {
		t.Parallel()

		ownerQuery := &recordingContentQuery{}
		updateQuery := &recordingContentQuery{}
		transaction := &recordingContentTx{
			queries: []contractsdb.Query{ownerQuery, updateQuery},
		}
		repository := NewContentRepository(
			&recordingContentDB{transaction: transaction},
		)

		err := repository.DeletePost(
			context.Background(),
			contentRepositoryUserID,
			contentRepositoryPostID,
		)

		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("DeletePost error = %v, want ErrNotFound", err)
		}
		if len(updateQuery.updates) != 0 {
			t.Fatalf("missing post must not update: %#v", updateQuery.updates)
		}
	})
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}

	return false
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
	exam_name,
	status,
	published_at
) VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP)
RETURNING id`
	if transaction.selects[0].query != wantQuery {
		t.Fatalf("insert query = %q, want %q", transaction.selects[0].query, wantQuery)
	}

	wantArgs := []any{
		contentRepositoryUserID,
		input.Title,
		input.Caption,
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
		getID:     contentRepositoryReactionID,
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
	if query.countCalls != 0 {
		t.Fatalf("locking existence check used Count %d times, want 0", query.countCalls)
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

	query := &recordingContentQuery{getID: contentRepositoryPostID}
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
	if query.countCalls != 0 {
		t.Fatalf("parent post lock used Count %d times, want 0", query.countCalls)
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

func TestLoadCommentCountsUsesOneFilteredBatchQuery(t *testing.T) {
	t.Parallel()

	query := &recordingContentQuery{
		getRows: func(destination any) {
			rows := destination.(*[]commentCountRow)
			*rows = append(*rows,
				commentCountRow{PostID: contentRepositoryPostID, Total: 3},
				commentCountRow{PostID: contentRepositoryNewPost, Total: 1},
			)
		},
	}
	transaction := &recordingContentTx{query: query}

	counts, err := loadCommentCounts(
		transaction,
		[]string{contentRepositoryPostID, contentRepositoryNewPost},
	)

	if err != nil {
		t.Fatalf("loadCommentCounts error = %v", err)
	}
	if !reflect.DeepEqual(counts, map[string]int64{
		contentRepositoryPostID:  3,
		contentRepositoryNewPost: 1,
	}) {
		t.Fatalf("comment counts = %#v", counts)
	}
	if !reflect.DeepEqual(query.selects, []string{"post_id", "COUNT(*) AS total"}) {
		t.Fatalf("selected columns = %#v", query.selects)
	}
	if !reflect.DeepEqual(query.whereIns, []recordedWhere{{
		query: "post_id",
		args:  []any{contentRepositoryPostID, contentRepositoryNewPost},
	}}) {
		t.Fatalf("post ID batch filter = %#v", query.whereIns)
	}
	if !reflect.DeepEqual(query.wheres, []recordedWhere{{
		query: "status",
		args:  []any{models.CommentStatusVisible},
	}}) {
		t.Fatalf("visibility filter = %#v", query.wheres)
	}
	if !reflect.DeepEqual(query.whereNulls, []string{"deleted_at"}) {
		t.Fatalf("soft-delete filters = %#v", query.whereNulls)
	}
	if !reflect.DeepEqual(query.groupBy, []string{"post_id"}) {
		t.Fatalf("grouping = %#v", query.groupBy)
	}
}

func TestPostExposesCamelCaseCommentCount(t *testing.T) {
	t.Parallel()

	field, ok := reflect.TypeOf(Post{}).FieldByName("CommentCount")
	if !ok {
		t.Fatal("Post must expose CommentCount")
	}
	if field.Tag.Get("json") != "commentCount" {
		t.Fatalf("CommentCount JSON tag = %q, want commentCount", field.Tag.Get("json"))
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
