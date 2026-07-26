package ai

import (
	"context"
	"errors"
	"testing"
	"time"

	contractsai "github.com/goravel/framework/contracts/ai"

	"goravel/app/repositories"
	"goravel/app/services"
)

type extractorTopicResolverStub struct{}

func (*extractorTopicResolverStub) ResolveTopic(context.Context, string) (repositories.AssistantTopic, bool, error) {
	return repositories.AssistantTopic{}, false, nil
}

func TestOpenAITopicExtractorUsesGoravelAgent(t *testing.T) {
	t.Parallel()

	response := &agentResponseFake{text: "phong cảnh"}
	conversation := &conversationFake{response: response}
	client := &aiClientFake{conversation: conversation}
	extractor := NewOpenAITopicExtractor(client, &extractorTopicResolverStub{})

	candidate, err := extractor.Extract(context.Background(), "Có bao nhiêu bài về phong cảnh?")

	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if candidate != "phong cảnh" {
		t.Fatalf("Extract() = %q, want %q", candidate, "phong cảnh")
	}
	if !client.withContextCalled || client.agent == nil || conversation.prompt == "" ||
		conversation.prompt == "Có bao nhiêu bài về phong cảnh?" {
		t.Fatalf("Goravel agent call was incomplete: %#v %#v", client, conversation)
	}
}

func TestOpenAITopicExtractorPropagatesProviderFailureForLocalFallback(t *testing.T) {
	t.Parallel()

	providerFailure := errors.New("openai unavailable")
	conversation := &conversationFake{err: providerFailure}
	client := &aiClientFake{conversation: conversation}
	extractor := NewOpenAITopicExtractor(client, &extractorTopicResolverStub{})

	_, err := extractor.Extract(context.Background(), "Đếm bài về phong cảnh")

	if !errors.Is(err, providerFailure) {
		t.Fatalf("Extract() error = %v, want provider failure", err)
	}
}

func TestOpenAITopicExtractorTimeoutLeavesTimeForLocalFallback(t *testing.T) {
	t.Parallel()

	const applicationRequestTimeout = 3 * time.Second
	if openAITopicExtractionTimeout >= applicationRequestTimeout {
		t.Fatalf(
			"OpenAI timeout = %s, must be shorter than HTTP request timeout %s",
			openAITopicExtractionTimeout,
			applicationRequestTimeout,
		)
	}
}

func TestConfiguredOpenAITopicExtractorReturnsNilDependencyWhenOpenAIIsDisabled(
	t *testing.T,
) {
	t.Parallel()

	clientFactoryCalled := false
	var extractor services.TopicExtractor = newConfiguredOpenAITopicExtractor(
		"local",
		"unused-api-key",
		func() contractsai.AI {
			clientFactoryCalled = true
			return &aiClientFake{}
		},
		&extractorTopicResolverStub{},
	)

	if extractor != nil {
		t.Fatalf("configured extractor = %#v, want nil", extractor)
	}
	if clientFactoryCalled {
		t.Fatal("AI client factory was called while OpenAI was disabled")
	}
}

func TestConfiguredOpenAITopicExtractorReturnsNilDependencyWhenAPIKeyIsMissing(
	t *testing.T,
) {
	t.Parallel()

	clientFactoryCalled := false
	var extractor services.TopicExtractor = newConfiguredOpenAITopicExtractor(
		"openai",
		"   ",
		func() contractsai.AI {
			clientFactoryCalled = true
			return &aiClientFake{}
		},
		&extractorTopicResolverStub{},
	)

	if extractor != nil {
		t.Fatalf("configured extractor = %#v, want nil", extractor)
	}
	if clientFactoryCalled {
		t.Fatal("AI client factory was called without an OpenAI API key")
	}
}

func TestConfiguredOpenAITopicExtractorBuildsDependencyWhenOpenAIIsEnabled(
	t *testing.T,
) {
	t.Parallel()

	client := &aiClientFake{}
	var extractor services.TopicExtractor = newConfiguredOpenAITopicExtractor(
		"openai",
		"test-api-key",
		func() contractsai.AI {
			return client
		},
		&extractorTopicResolverStub{},
	)

	configured, ok := extractor.(*OpenAITopicExtractor)
	if !ok {
		t.Fatalf("configured extractor = %T, want *OpenAITopicExtractor", extractor)
	}
	if configured.client != client {
		t.Fatal("configured extractor did not keep the resolved AI client")
	}
}

type aiClientFake struct {
	contractsai.AI
	conversation      contractsai.Conversation
	err               error
	withContextCalled bool
	agent             contractsai.Agent
	options           []contractsai.Option
}

func (f *aiClientFake) WithContext(context.Context) contractsai.AI {
	f.withContextCalled = true
	return f
}

func (f *aiClientFake) Agent(agent contractsai.Agent, options ...contractsai.Option) (contractsai.Conversation, error) {
	f.agent = agent
	f.options = options
	return f.conversation, f.err
}

type conversationFake struct {
	contractsai.Conversation
	response contractsai.AgentResponse
	err      error
	prompt   string
}

func (f *conversationFake) Prompt(input string, _ ...contractsai.ConversationOption) (contractsai.AgentResponse, error) {
	f.prompt = input
	return f.response, f.err
}

type agentResponseFake struct {
	contractsai.AgentResponse
	text string
}

func (f *agentResponseFake) Text() string {
	return f.text
}
