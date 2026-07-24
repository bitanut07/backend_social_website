package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	"goravel/app/repositories"
)

type topicResolverFake struct {
	topic      repositories.AssistantTopic
	found      bool
	err        error
	normalized string
}

func (f *topicResolverFake) ResolveTopic(_ context.Context, normalized string) (repositories.AssistantTopic, bool, error) {
	f.normalized = normalized
	return f.topic, f.found, f.err
}

func TestTopicLookupToolResolvesOnlyNormalizedAllowlistedTopic(t *testing.T) {
	t.Parallel()

	resolver := &topicResolverFake{
		topic: repositories.AssistantTopic{
			ID:   2,
			Slug: "phong-canh",
			Name: "Phong cảnh",
		},
		found: true,
	}
	tool := NewTopicLookupTool(resolver)

	result, err := tool.Execute(context.Background(), map[string]any{
		"topic": `  CẢNH VẬT" OR 1=1 -- `,
	})

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if resolver.normalized != "canh vat or 1 1" {
		t.Fatalf("ResolveTopic() candidate = %q", resolver.normalized)
	}
	if result != `{"found":true,"topic":{"id":2,"slug":"phong-canh","name":"Phong cảnh"}}` {
		t.Fatalf("Execute() result = %q", result)
	}
}

func TestTopicLookupToolReturnsSafeNotFoundResult(t *testing.T) {
	t.Parallel()

	resolver := &topicResolverFake{}
	tool := NewTopicLookupTool(resolver)

	result, err := tool.Execute(context.Background(), map[string]any{"topic": "không tồn tại"})

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result != `{"found":false}` {
		t.Fatalf("Execute() result = %q", result)
	}
}

func TestTopicLookupToolRejectsMissingTopic(t *testing.T) {
	t.Parallel()

	tool := NewTopicLookupTool(&topicResolverFake{})

	_, err := tool.Execute(context.Background(), map[string]any{})

	if !errors.Is(err, ErrInvalidTopicArgument) {
		t.Fatalf("Execute() error = %v, want ErrInvalidTopicArgument", err)
	}
}

func TestTopicLookupToolCapsCandidateAtOneHundredCharacters(t *testing.T) {
	t.Parallel()

	resolver := &topicResolverFake{}
	tool := NewTopicLookupTool(resolver)

	_, err := tool.Execute(context.Background(), map[string]any{"topic": strings.Repeat("a", 140)})

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len([]rune(resolver.normalized)) != 100 {
		t.Fatalf("ResolveTopic() candidate length = %d, want 100", len([]rune(resolver.normalized)))
	}
}
