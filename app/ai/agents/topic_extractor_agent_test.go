package agents

import (
	"context"
	"strings"
	"testing"

	"goravel/app/repositories"
)

type topicResolverStub struct{}

func (*topicResolverStub) ResolveTopic(context.Context, string) (repositories.AssistantTopic, bool, error) {
	return repositories.AssistantTopic{}, false, nil
}

func TestTopicExtractorAgentExposesOnlySafeLookupTool(t *testing.T) {
	t.Parallel()

	agent := NewTopicExtractorAgent(&topicResolverStub{})
	tools := agent.Tools()

	if len(tools) != 1 || tools[0].Name() != "resolve_topic" {
		t.Fatalf("Tools() = %#v", tools)
	}
	instructions := strings.ToLower(agent.Instructions())
	if !strings.Contains(instructions, "không được tạo sql") ||
		!strings.Contains(instructions, "chỉ trả về cụm chủ đề") ||
		!strings.Contains(instructions, "không xác định lại ý định") {
		t.Fatalf("Instructions() = %q", agent.Instructions())
	}
	if agent.Messages() != nil || agent.Middleware() != nil {
		t.Fatal("agent unexpectedly carries conversation history or middleware")
	}
}
