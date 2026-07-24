package feature

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"goravel/app/facades"
	"goravel/tests"
)

type HealthTestSuite struct {
	suite.Suite
	tests.TestCase
}

func TestHealthTestSuite(t *testing.T) {
	suite.Run(t, new(HealthTestSuite))
}

// SetupTest will run before each test in the suite.
func (s *HealthTestSuite) SetupTest() {
}

// TearDownTest will run after each test in the suite.
func (s *HealthTestSuite) TearDownTest() {
}

func (s *HealthTestSuite) TestHealth() {
	response, err := s.Http(s.T()).Get("/api/v1/health")

	s.Require().NoError(err)
	response.
		AssertOk().
		AssertExactJson(map[string]any{
			"status": "OK",
		})
}

func (s *HealthTestSuite) TestChunkedPayloadIsRejectedBeforeController() {
	request, err := http.NewRequest(
		http.MethodPost,
		"/api/v1/messages",
		strings.NewReader(strings.Repeat("x", 65*1024)),
	)
	s.Require().NoError(err)
	request.ContentLength = -1
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-User-ID", "00000000-0000-4000-8000-000000000001")

	response, err := facades.Route().Test(request)
	s.Require().NoError(err)
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	s.Require().NoError(err)
	s.Equal(http.StatusRequestEntityTooLarge, response.StatusCode)
	s.JSONEq(
		`{"error":{"code":"PAYLOAD_TOO_LARGE","message":"Dữ liệu gửi lên vượt quá giới hạn cho phép","details":{}}}`,
		string(body),
	)
}
