package services_test

import (
	"context"
	"os"
	"testing"
	"time"

	appai "goravel/app/ai"
	"goravel/app/facades"
	"goravel/app/repositories"
	"goravel/app/services"
	"goravel/bootstrap"
)

// TestLiveAssistantService is opt-in because it uses the configured database
// and providers. It logs no credentials, prompts or connection strings.
func TestLiveAssistantService(t *testing.T) {
	if os.Getenv("ASSISTANT_SERVICE_LIVE_TEST") != "1" {
		t.Skip("set ASSISTANT_SERVICE_LIVE_TEST=1 to use configured services")
	}

	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir("../.."); err != nil {
		t.Fatalf("change to backend directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(workingDirectory)
	})

	bootstrap.Boot()
	repository := repositories.NewAssistantRepository(facades.Orm())
	extractor := appai.NewConfiguredOpenAITopicExtractor(repository)
	responder := appai.NewConfiguredModelLLMAssistant()
	var topicExtractor services.TopicExtractor
	if extractor != nil {
		topicExtractor = extractor
	}
	service := services.NewAssistantService(repository, topicExtractor)
	if responder != nil {
		service = services.NewAssistantService(
			repository,
			topicExtractor,
			responder,
		)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	response, err := service.Ask(
		ctx,
		"8b484b4f-c468-49f6-a2b7-cdd7e2bfc380",
		"Có bao nhiêu bài về cà phê?",
	)
	if err != nil {
		t.Fatalf("AssistantService.Ask() live error: %v", err)
	}
	if response.Status != services.AssistantStatusAnswered ||
		response.Result == nil ||
		response.Result.Count != 1 {
		t.Fatalf("AssistantService.Ask() live response = %#v", response)
	}
	t.Logf(
		"live provider=%s topic=%q count=%d",
		response.Provider,
		response.Result.Topic.Name,
		response.Result.Count,
	)
}
