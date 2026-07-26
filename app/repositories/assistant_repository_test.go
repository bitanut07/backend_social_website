package repositories

import (
	"context"
	"errors"
	"reflect"
	"testing"

	frameworkerrors "github.com/goravel/framework/errors"

	"goravel/app/models"
)

const (
	assistantRepositoryTestUserID           = "00000000-0000-4000-8000-000000000001"
	assistantRepositoryTestLandscapeTopicID = "10000000-0000-4000-8000-000000000001"
)

type assistantQueryCall struct {
	method string
	value  any
	args   []any
}

type assistantQueryFake struct {
	calls      []assistantQueryCall
	firstTopic models.Topic
	firstErr   error
	aliases    []models.TopicAlias
	getErr     error
	count      int64
	countErr   error
	exists     bool
	existsErr  error
}

func (f *assistantQueryFake) Model(value any) assistantQuery {
	f.calls = append(f.calls, assistantQueryCall{method: "Model", value: value})
	return f
}

func (f *assistantQueryFake) Join(query string, args ...any) assistantQuery {
	f.calls = append(f.calls, assistantQueryCall{method: "Join", value: query, args: args})
	return f
}

func (f *assistantQueryFake) Where(query any, args ...any) assistantQuery {
	f.calls = append(f.calls, assistantQueryCall{method: "Where", value: query, args: args})
	return f
}

func (f *assistantQueryFake) OrderBy(column string, direction ...string) assistantQuery {
	args := make([]any, len(direction))
	for index, value := range direction {
		args[index] = value
	}
	f.calls = append(f.calls, assistantQueryCall{method: "OrderBy", value: column, args: args})
	return f
}

func (f *assistantQueryFake) Distinct(columns ...string) assistantQuery {
	args := make([]any, len(columns))
	for index, value := range columns {
		args[index] = value
	}
	f.calls = append(f.calls, assistantQueryCall{method: "Distinct", args: args})
	return f
}

func (f *assistantQueryFake) FirstOrFail(destination any) error {
	f.calls = append(f.calls, assistantQueryCall{method: "FirstOrFail", value: destination})
	if f.firstErr != nil {
		return f.firstErr
	}
	*(destination.(*models.Topic)) = f.firstTopic
	return nil
}

func (f *assistantQueryFake) Get(destination any) error {
	f.calls = append(f.calls, assistantQueryCall{method: "Get", value: destination})
	if f.getErr != nil {
		return f.getErr
	}
	*(destination.(*[]models.TopicAlias)) = f.aliases
	return nil
}

func (f *assistantQueryFake) Count() (int64, error) {
	f.calls = append(f.calls, assistantQueryCall{method: "Count"})
	return f.count, f.countErr
}

func (f *assistantQueryFake) Exists() (bool, error) {
	f.calls = append(f.calls, assistantQueryCall{method: "Exists"})
	return f.exists, f.existsErr
}

type assistantDatabaseFake struct {
	queries           []assistantQuery
	withContextCalled bool
}

func (f *assistantDatabaseFake) WithContext(context.Context) assistantDatabase {
	f.withContextCalled = true
	return f
}

func (f *assistantDatabaseFake) Query() assistantQuery {
	query := f.queries[0]
	f.queries = f.queries[1:]
	return query
}

func TestAssistantRepositoryResolveTopicByCanonicalName(t *testing.T) {
	t.Parallel()

	normalized := `phong canh" OR 1=1 --`
	topicQuery := &assistantQueryFake{
		firstTopic: models.Topic{
			BaseModel:      models.BaseModel{ID: assistantRepositoryTestLandscapeTopicID},
			Slug:           "phong-canh",
			Name:           "Phong cảnh",
			NormalizedName: "phong canh",
		},
	}
	aliasQuery := &assistantQueryFake{
		aliases: []models.TopicAlias{
			{Alias: "cảnh vật"},
			{Alias: "landscape"},
		},
	}
	database := &assistantDatabaseFake{queries: []assistantQuery{topicQuery, aliasQuery}}
	repository := newAssistantRepository(database)

	topic, found, err := repository.ResolveTopic(context.Background(), normalized)

	if err != nil {
		t.Fatalf("ResolveTopic() error = %v", err)
	}
	if !found {
		t.Fatal("ResolveTopic() found = false, want true")
	}
	if topic.ID != assistantRepositoryTestLandscapeTopicID ||
		topic.Slug != "phong-canh" ||
		topic.Name != "Phong cảnh" {
		t.Fatalf("ResolveTopic() topic = %#v", topic)
	}
	if len(topic.Aliases) != 2 || topic.Aliases[0] != "cảnh vật" || topic.Aliases[1] != "landscape" {
		t.Fatalf("ResolveTopic() aliases = %#v", topic.Aliases)
	}
	assertAssistantQueryCall(
		t,
		topicQuery.calls,
		"Where",
		"normalized_name IN (?, ?)",
		[]any{normalized, `phong-canh"-OR-1=1---`},
	)
	assertAssistantQueryCall(
		t,
		aliasQuery.calls,
		"Where",
		"topic_id = ?",
		[]any{assistantRepositoryTestLandscapeTopicID},
	)
}

func TestAssistantRepositoryResolveTopicByAlias(t *testing.T) {
	t.Parallel()

	canonicalQuery := &assistantQueryFake{firstErr: frameworkerrors.OrmRecordNotFound}
	aliasLookupQuery := &assistantQueryFake{
		firstTopic: models.Topic{
			BaseModel:      models.BaseModel{ID: assistantRepositoryTestLandscapeTopicID},
			Slug:           "phong-canh",
			Name:           "Phong cảnh",
			NormalizedName: "phong canh",
		},
	}
	aliasesQuery := &assistantQueryFake{}
	database := &assistantDatabaseFake{
		queries: []assistantQuery{canonicalQuery, aliasLookupQuery, aliasesQuery},
	}
	repository := newAssistantRepository(database)

	topic, found, err := repository.ResolveTopic(context.Background(), "canh vat")

	if err != nil {
		t.Fatalf("ResolveTopic() error = %v", err)
	}
	if !found || topic.ID != assistantRepositoryTestLandscapeTopicID {
		t.Fatalf(
			"ResolveTopic() = (%#v, %v), want topic %s",
			topic,
			found,
			assistantRepositoryTestLandscapeTopicID,
		)
	}
	assertAssistantQueryCall(
		t,
		aliasLookupQuery.calls,
		"Join",
		"JOIN topic_aliases ON topic_aliases.topic_id = topics.id",
		nil,
	)
	assertAssistantQueryCall(
		t,
		aliasLookupQuery.calls,
		"Where",
		"topic_aliases.normalized_alias IN (?, ?)",
		[]any{"canh vat", "canh-vat"},
	)
}

func TestAssistantRepositoryReturnsNotFoundForUnknownTopic(t *testing.T) {
	t.Parallel()

	canonicalQuery := &assistantQueryFake{firstErr: frameworkerrors.OrmRecordNotFound}
	aliasQuery := &assistantQueryFake{firstErr: frameworkerrors.OrmRecordNotFound}
	database := &assistantDatabaseFake{queries: []assistantQuery{canonicalQuery, aliasQuery}}
	repository := newAssistantRepository(database)

	topic, found, err := repository.ResolveTopic(context.Background(), "khong ton tai")

	if err != nil {
		t.Fatalf("ResolveTopic() error = %v", err)
	}
	if found || topic.ID != "" {
		t.Fatalf("ResolveTopic() = (%#v, %v), want not found", topic, found)
	}
}

func TestAssistantRepositoryCountPublishedPostsUsesDistinctAndParameters(t *testing.T) {
	t.Parallel()

	query := &assistantQueryFake{count: 7}
	database := &assistantDatabaseFake{queries: []assistantQuery{query}}
	repository := newAssistantRepository(database)

	count, err := repository.CountPublishedPostsByTopic(
		context.Background(),
		assistantRepositoryTestLandscapeTopicID,
	)

	if err != nil {
		t.Fatalf("CountPublishedPostsByTopic() error = %v", err)
	}
	if count != 7 {
		t.Fatalf("CountPublishedPostsByTopic() = %d, want 7", count)
	}
	assertAssistantQueryCall(
		t,
		query.calls,
		"Join",
		"JOIN post_topics ON post_topics.post_id = posts.id",
		nil,
	)
	assertAssistantQueryCall(
		t,
		query.calls,
		"Where",
		"post_topics.topic_id = ?",
		[]any{assistantRepositoryTestLandscapeTopicID},
	)
	assertAssistantQueryCall(t, query.calls, "Where", "posts.status = ?", []any{models.PostStatusPublished})
	assertAssistantQueryCall(t, query.calls, "Distinct", nil, []any{"posts.id"})
}

func TestAssistantRepositoryPropagatesResolveErrors(t *testing.T) {
	t.Parallel()

	databaseFailure := errors.New("database unavailable")
	query := &assistantQueryFake{firstErr: databaseFailure}
	database := &assistantDatabaseFake{queries: []assistantQuery{query}}
	repository := newAssistantRepository(database)

	_, _, err := repository.ResolveTopic(context.Background(), "phong canh")

	if !errors.Is(err, databaseFailure) {
		t.Fatalf("ResolveTopic() error = %v, want database failure", err)
	}
}

func TestAssistantRepositoryChecksDemoUserWithBoundID(t *testing.T) {
	t.Parallel()

	query := &assistantQueryFake{exists: true}
	database := &assistantDatabaseFake{queries: []assistantQuery{query}}
	repository := newAssistantRepository(database)

	exists, err := repository.UserExists(context.Background(), assistantRepositoryTestUserID)

	if err != nil || !exists {
		t.Fatalf("UserExists() = (%v, %v), want (true, nil)", exists, err)
	}
	assertAssistantQueryCall(
		t,
		query.calls,
		"Where",
		"id = ?",
		[]any{assistantRepositoryTestUserID},
	)
}

func assertAssistantQueryCall(
	t *testing.T,
	calls []assistantQueryCall,
	method string,
	value any,
	args []any,
) {
	t.Helper()

	for _, call := range calls {
		if call.method == method && reflect.DeepEqual(call.value, value) && reflect.DeepEqual(call.args, args) {
			return
		}
	}

	t.Fatalf("query call %s(%#v, %#v) not found in %#v", method, value, args, calls)
}
