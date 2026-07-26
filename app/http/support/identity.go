package support

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	stdhttp "net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/goravel/framework/contracts/http"

	"goravel/app/facades"
)

var ErrMissingUserID = errors.New("phiên đăng nhập không hợp lệ hoặc đã hết hạn")

var ErrInvalidAccessToken = errors.New("access token Supabase không hợp lệ")

var ErrInvalidResourceID = errors.New("ID phải là một UUID hợp lệ")

var supabaseAuthHTTPClient = &stdhttp.Client{Timeout: 5 * time.Second}

func ParseResourceID(value string) (string, error) {
	candidate := strings.TrimSpace(value)
	id, err := uuid.Parse(candidate)
	if err != nil ||
		id == uuid.Nil ||
		len(candidate) != len(uuid.Nil.String()) ||
		!strings.EqualFold(candidate, id.String()) {
		return "", ErrInvalidResourceID
	}

	return id.String(), nil
}

func ParseUserIDHeader(value string) (string, error) {
	id, err := ParseResourceID(value)
	if err != nil {
		return "", ErrMissingUserID
	}

	return id, nil
}

func ParseBearerToken(value string) (string, error) {
	parts := strings.Fields(value)
	if len(parts) != 2 ||
		!strings.EqualFold(parts[0], "Bearer") ||
		strings.TrimSpace(parts[1]) == "" {
		return "", ErrInvalidAccessToken
	}

	return parts[1], nil
}

func VerifySupabaseAccessToken(
	token string,
	supabaseURL string,
	publishableKey string,
	client *stdhttp.Client,
) (string, error) {
	baseURL, err := url.Parse(strings.TrimSpace(supabaseURL))
	if err != nil ||
		baseURL.Host == "" ||
		(baseURL.Scheme != "https" &&
			baseURL.Hostname() != "127.0.0.1" &&
			baseURL.Hostname() != "localhost") {
		return "", ErrInvalidAccessToken
	}
	if strings.TrimSpace(publishableKey) == "" || strings.TrimSpace(token) == "" {
		return "", ErrInvalidAccessToken
	}

	endpoint, err := baseURL.Parse("/auth/v1/user")
	if err != nil {
		return "", ErrInvalidAccessToken
	}
	request, err := stdhttp.NewRequest(stdhttp.MethodGet, endpoint.String(), nil)
	if err != nil {
		return "", ErrInvalidAccessToken
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("apikey", publishableKey)

	if client == nil {
		client = supabaseAuthHTTPClient
	}
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("%w: không thể xác minh phiên", ErrInvalidAccessToken)
	}
	defer response.Body.Close()

	if response.StatusCode != stdhttp.StatusOK {
		return "", ErrInvalidAccessToken
	}

	var payload struct {
		ID string `json:"id"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 64*1024))
	if err := decoder.Decode(&payload); err != nil {
		return "", ErrInvalidAccessToken
	}

	userID, err := ParseResourceID(payload.ID)
	if err != nil {
		return "", ErrInvalidAccessToken
	}
	return userID, nil
}

func CurrentUserID(ctx http.Context) (string, error) {
	authorization := strings.TrimSpace(ctx.Request().Header("Authorization"))
	if authorization != "" {
		token, err := ParseBearerToken(authorization)
		if err != nil {
			return "", err
		}
		config := facades.Config()
		return VerifySupabaseAccessToken(
			token,
			config.GetString("supabase.url"),
			config.GetString("supabase.publishable_key"),
			nil,
		)
	}

	config := facades.Config()
	environment := strings.ToLower(config.GetString("app.env"))
	if environment != "local" &&
		environment != "testing" &&
		config.GetString("supabase.url") != "" &&
		!config.GetBool("supabase.allow_demo_user_header") {
		return "", ErrMissingUserID
	}

	return ParseUserIDHeader(ctx.Request().Header("X-User-ID"))
}
