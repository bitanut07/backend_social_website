package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"goravel/app/repositories"
)

const maximumToolTopicLength = 100

var ErrInvalidTopicArgument = errors.New("topic must be a non-empty string")

type TopicResolver interface {
	ResolveTopic(context.Context, string) (repositories.AssistantTopic, bool, error)
}

type TopicLookupTool struct {
	resolver TopicResolver
}

func NewTopicLookupTool(resolver TopicResolver) *TopicLookupTool {
	return &TopicLookupTool{resolver: resolver}
}

func (t *TopicLookupTool) Name() string {
	return "resolve_topic"
}

func (t *TopicLookupTool) Description() string {
	return "Kiểm tra một cụm chủ đề trong danh mục cho phép; công cụ không nhận hoặc thực thi SQL."
}

func (t *TopicLookupTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"topic": map[string]any{
				"type":        "string",
				"description": "Cụm chủ đề thuần văn bản cần kiểm tra.",
				"minLength":   1,
				"maxLength":   maximumToolTopicLength,
			},
		},
		"required":             []string{"topic"},
		"additionalProperties": false,
	}
}

func (t *TopicLookupTool) Execute(
	ctx context.Context,
	arguments map[string]any,
) (string, error) {
	rawTopic, ok := arguments["topic"].(string)
	if !ok {
		return "", ErrInvalidTopicArgument
	}

	normalized := normalizeToolTopic(rawTopic)
	if normalized == "" {
		return "", ErrInvalidTopicArgument
	}

	topic, found, err := t.resolver.ResolveTopic(ctx, normalized)
	if err != nil {
		return "", err
	}
	if !found {
		return `{"found":false}`, nil
	}

	result, err := json.Marshal(struct {
		Found bool `json:"found"`
		Topic struct {
			ID   uint64 `json:"id"`
			Slug string `json:"slug"`
			Name string `json:"name"`
		} `json:"topic"`
	}{
		Found: true,
		Topic: struct {
			ID   uint64 `json:"id"`
			Slug string `json:"slug"`
			Name string `json:"name"`
		}{
			ID:   topic.ID,
			Slug: topic.Slug,
			Name: topic.Name,
		},
	})
	if err != nil {
		return "", err
	}

	return string(result), nil
}

func normalizeToolTopic(value string) string {
	value = strings.ReplaceAll(value, "Đ", "D")
	value = strings.ReplaceAll(value, "đ", "d")
	value = norm.NFD.String(strings.ToLower(value))

	var normalized strings.Builder
	normalized.Grow(len(value))
	for _, character := range value {
		switch {
		case unicode.Is(unicode.Mn, character):
			continue
		case unicode.IsLetter(character), unicode.IsDigit(character):
			normalized.WriteRune(character)
		default:
			normalized.WriteByte(' ')
		}
	}

	runes := []rune(strings.Join(strings.Fields(normalized.String()), " "))
	if len(runes) > maximumToolTopicLength {
		runes = runes[:maximumToolTopicLength]
	}

	return strings.TrimSpace(string(runes))
}
