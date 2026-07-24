package repositories

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/goravel/framework/contracts/database/orm"
	frameworkerrors "github.com/goravel/framework/errors"

	"goravel/app/models"
)

const maximumNormalizedTopicLength = 100

type AssistantTopic struct {
	ID      uint64
	Slug    string
	Name    string
	Aliases []string
}

type AssistantRepository struct {
	database assistantDatabase
}

func NewAssistantRepository(database orm.Orm) *AssistantRepository {
	return newAssistantRepository(&goravelAssistantDatabase{database: database})
}

func newAssistantRepository(database assistantDatabase) *AssistantRepository {
	return &AssistantRepository{database: database}
}

func (r *AssistantRepository) UserExists(ctx context.Context, userID uint64) (bool, error) {
	return r.database.
		WithContext(ctx).
		Query().
		Model(&models.User{}).
		Where("id = ?", userID).
		Exists()
}

func (r *AssistantRepository) ResolveTopic(
	ctx context.Context,
	normalized string,
) (AssistantTopic, bool, error) {
	normalized = strings.TrimSpace(normalized)
	if normalized == "" || utf8.RuneCountInString(normalized) > maximumNormalizedTopicLength {
		return AssistantTopic{}, false, nil
	}

	database := r.database.WithContext(ctx)
	topic, found, err := findCanonicalTopic(database.Query(), normalized)
	if err != nil {
		return AssistantTopic{}, false, err
	}

	if !found {
		topic, found, err = findTopicByAlias(database.Query(), normalized)
		if err != nil || !found {
			return AssistantTopic{}, found, err
		}
	}

	aliases, err := listTopicAliases(database.Query(), topic.ID)
	if err != nil {
		return AssistantTopic{}, false, err
	}

	return AssistantTopic{
		ID:      topic.ID,
		Slug:    topic.Slug,
		Name:    topic.Name,
		Aliases: aliases,
	}, true, nil
}

func (r *AssistantRepository) CountPublishedPostsByTopic(
	ctx context.Context,
	topicID uint64,
) (int64, error) {
	return r.database.
		WithContext(ctx).
		Query().
		Model(&models.Post{}).
		Join("JOIN post_topics ON post_topics.post_id = posts.id").
		Where("post_topics.topic_id = ?", topicID).
		Where("posts.status = ?", models.PostStatusPublished).
		Distinct("posts.id").
		Count()
}

func findCanonicalTopic(query assistantQuery, normalized string) (models.Topic, bool, error) {
	var topic models.Topic
	err := query.
		Model(&models.Topic{}).
		Where("normalized_name = ?", normalized).
		FirstOrFail(&topic)
	if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
		return models.Topic{}, false, nil
	}
	if err != nil {
		return models.Topic{}, false, err
	}

	return topic, true, nil
}

func findTopicByAlias(query assistantQuery, normalized string) (models.Topic, bool, error) {
	var topic models.Topic
	err := query.
		Model(&models.Topic{}).
		Join("JOIN topic_aliases ON topic_aliases.topic_id = topics.id").
		Where("topic_aliases.normalized_alias = ?", normalized).
		FirstOrFail(&topic)
	if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
		return models.Topic{}, false, nil
	}
	if err != nil {
		return models.Topic{}, false, err
	}

	return topic, true, nil
}

func listTopicAliases(query assistantQuery, topicID uint64) ([]string, error) {
	var aliases []models.TopicAlias
	err := query.
		Model(&models.TopicAlias{}).
		Where("topic_id = ?", topicID).
		OrderBy("alias").
		Get(&aliases)
	if err != nil {
		return nil, err
	}

	values := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		values = append(values, alias.Alias)
	}

	return values, nil
}

type assistantDatabase interface {
	WithContext(context.Context) assistantDatabase
	Query() assistantQuery
}

type assistantQuery interface {
	Model(any) assistantQuery
	Join(string, ...any) assistantQuery
	Where(any, ...any) assistantQuery
	OrderBy(string, ...string) assistantQuery
	Distinct(...string) assistantQuery
	FirstOrFail(any) error
	Get(any) error
	Count() (int64, error)
	Exists() (bool, error)
}

type goravelAssistantDatabase struct {
	database orm.Orm
}

func (d *goravelAssistantDatabase) WithContext(ctx context.Context) assistantDatabase {
	return &goravelAssistantDatabase{database: d.database.WithContext(ctx)}
}

func (d *goravelAssistantDatabase) Query() assistantQuery {
	return &goravelAssistantQuery{query: d.database.Query()}
}

type goravelAssistantQuery struct {
	query orm.Query
}

func (q *goravelAssistantQuery) Model(value any) assistantQuery {
	return &goravelAssistantQuery{query: q.query.Model(value)}
}

func (q *goravelAssistantQuery) Join(query string, args ...any) assistantQuery {
	return &goravelAssistantQuery{query: q.query.Join(query, args...)}
}

func (q *goravelAssistantQuery) Where(query any, args ...any) assistantQuery {
	return &goravelAssistantQuery{query: q.query.Where(query, args...)}
}

func (q *goravelAssistantQuery) OrderBy(column string, direction ...string) assistantQuery {
	return &goravelAssistantQuery{query: q.query.OrderBy(column, direction...)}
}

func (q *goravelAssistantQuery) Distinct(columns ...string) assistantQuery {
	return &goravelAssistantQuery{query: q.query.Distinct(columns...)}
}

func (q *goravelAssistantQuery) FirstOrFail(destination any) error {
	return q.query.FirstOrFail(destination)
}

func (q *goravelAssistantQuery) Get(destination any) error {
	return q.query.Get(destination)
}

func (q *goravelAssistantQuery) Count() (int64, error) {
	return q.query.Count()
}

func (q *goravelAssistantQuery) Exists() (bool, error) {
	return q.query.Exists()
}
