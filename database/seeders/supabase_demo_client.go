package seeders

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxSupabaseResponseBytes = 2 * 1024 * 1024
const defaultDemoAuthPassword = "artly-demo"

type supabaseDemoClient struct {
	baseURL    string
	serviceKey string
	httpClient *http.Client
}

type supabaseAuthUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

func newSupabaseDemoClient(
	baseURL string,
	serviceKey string,
	httpClient *http.Client,
) *supabaseDemoClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	return &supabaseDemoClient{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		serviceKey: strings.TrimSpace(serviceKey),
		httpClient: httpClient,
	}
}

func (c *supabaseDemoClient) ensureAuthUsers(
	ctx context.Context,
	users []cafeDemoUser,
) (map[string]string, error) {
	existing, err := c.listAuthUsers(ctx)
	if err != nil {
		return nil, err
	}

	idsByEmail := make(map[string]string, len(existing)+len(users))
	for _, user := range existing {
		idsByEmail[strings.ToLower(user.Email)] = user.ID
	}

	for _, demoUser := range users {
		emailKey := strings.ToLower(demoUser.Email)
		if existingID := idsByEmail[emailKey]; existingID != "" {
			if err = c.updateAuthUser(ctx, existingID, demoUser); err != nil {
				return nil, err
			}
			continue
		}

		created, createErr := c.createAuthUser(ctx, demoUser)
		if createErr != nil {
			return nil, createErr
		}
		idsByEmail[emailKey] = created.ID
	}

	return idsByEmail, nil
}

func (c *supabaseDemoClient) listAuthUsers(
	ctx context.Context,
) ([]supabaseAuthUser, error) {
	request, err := c.newRequest(
		ctx,
		http.MethodGet,
		"/auth/v1/admin/users?page=1&per_page=1000",
		nil,
	)
	if err != nil {
		return nil, err
	}

	body, status, err := c.execute(request)
	if err != nil {
		return nil, fmt.Errorf("liệt kê tài khoản Supabase: %w", err)
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return nil, c.responseError("liệt kê tài khoản Supabase", status, body)
	}

	var response struct {
		Users []supabaseAuthUser `json:"users"`
	}
	if err = json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("đọc danh sách tài khoản Supabase: %w", err)
	}

	return response.Users, nil
}

func (c *supabaseDemoClient) createAuthUser(
	ctx context.Context,
	user cafeDemoUser,
) (supabaseAuthUser, error) {
	body, err := json.Marshal(demoAuthUserPayload(user))
	if err != nil {
		return supabaseAuthUser{}, fmt.Errorf("tạo payload tài khoản demo: %w", err)
	}

	request, err := c.newRequest(
		ctx,
		http.MethodPost,
		"/auth/v1/admin/users",
		bytes.NewReader(body),
	)
	if err != nil {
		return supabaseAuthUser{}, err
	}
	request.Header.Set("Content-Type", "application/json")

	responseBody, status, err := c.execute(request)
	if err != nil {
		return supabaseAuthUser{}, fmt.Errorf(
			"tạo tài khoản demo %s: %w",
			user.Email,
			err,
		)
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return supabaseAuthUser{}, c.responseError(
			"tạo tài khoản demo "+user.Email,
			status,
			responseBody,
		)
	}

	var response struct {
		ID    string            `json:"id"`
		Email string            `json:"email"`
		User  *supabaseAuthUser `json:"user"`
	}
	if err = json.Unmarshal(responseBody, &response); err != nil {
		return supabaseAuthUser{}, fmt.Errorf(
			"đọc tài khoản demo %s: %w",
			user.Email,
			err,
		)
	}
	if response.User != nil {
		return *response.User, nil
	}
	if response.ID == "" {
		return supabaseAuthUser{}, fmt.Errorf(
			"Supabase không trả về ID cho tài khoản demo %s",
			user.Email,
		)
	}

	return supabaseAuthUser{ID: response.ID, Email: response.Email}, nil
}

func (c *supabaseDemoClient) updateAuthUser(
	ctx context.Context,
	userID string,
	user cafeDemoUser,
) error {
	body, err := json.Marshal(demoAuthUserPayload(user))
	if err != nil {
		return fmt.Errorf("tạo payload cập nhật tài khoản demo: %w", err)
	}

	request, err := c.newRequest(
		ctx,
		http.MethodPut,
		"/auth/v1/admin/users/"+url.PathEscape(userID),
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")

	responseBody, status, err := c.execute(request)
	if err != nil {
		return fmt.Errorf(
			"cập nhật tài khoản demo %s: %w",
			user.Email,
			err,
		)
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return c.responseError(
			"cập nhật tài khoản demo "+user.Email,
			status,
			responseBody,
		)
	}

	return nil
}

func demoAuthUserPayload(user cafeDemoUser) map[string]any {
	return map[string]any{
		"email":         user.Email,
		"password":      demoAuthPassword(),
		"email_confirm": true,
		"user_metadata": map[string]any{
			"full_name":   user.DisplayName,
			"username":    user.Username,
			"accountType": user.Role,
		},
	}
}

func demoAuthPassword() string {
	if password := strings.TrimSpace(os.Getenv("ARTLY_DEMO_PASSWORD")); password != "" {
		return password
	}

	return defaultDemoAuthPassword
}

func (c *supabaseDemoClient) ensureDemoArtBucket(ctx context.Context) error {
	request, err := c.newRequest(
		ctx,
		http.MethodGet,
		"/storage/v1/bucket/demo-art",
		nil,
	)
	if err != nil {
		return err
	}

	body, status, err := c.execute(request)
	if err != nil {
		return fmt.Errorf("kiểm tra bucket demo-art: %w", err)
	}

	payload, err := json.Marshal(map[string]any{
		"id":                 "demo-art",
		"name":               "demo-art",
		"public":             true,
		"file_size_limit":    10 * 1024 * 1024,
		"allowed_mime_types": []string{"image/webp"},
	})
	if err != nil {
		return fmt.Errorf("tạo payload bucket demo-art: %w", err)
	}

	var method, path string
	switch status {
	case http.StatusOK:
		method = http.MethodPut
		path = "/storage/v1/bucket/demo-art"
	case http.StatusNotFound:
		method = http.MethodPost
		path = "/storage/v1/bucket"
	default:
		if !isMissingBucketResponse(status, body) {
			return c.responseError("kiểm tra bucket demo-art", status, body)
		}
		method = http.MethodPost
		path = "/storage/v1/bucket"
	}

	request, err = c.newRequest(ctx, method, path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")

	body, status, err = c.execute(request)
	if err != nil {
		return fmt.Errorf("cấu hình bucket demo-art: %w", err)
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return c.responseError("cấu hình bucket demo-art", status, body)
	}

	return nil
}

func (c *supabaseDemoClient) uploadDemoArtwork(
	ctx context.Context,
	fileName string,
	content []byte,
) error {
	if fileName == "" || filepath.Base(fileName) != fileName {
		return fmt.Errorf("tên ảnh demo không hợp lệ: %q", fileName)
	}

	request, err := c.newRequest(
		ctx,
		http.MethodPost,
		"/storage/v1/object/demo-art/"+url.PathEscape(fileName),
		bytes.NewReader(content),
	)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "image/webp")
	request.Header.Set("Cache-Control", "max-age=31536000")
	request.Header.Set("x-upsert", "true")

	body, status, err := c.execute(request)
	if err != nil {
		return fmt.Errorf("tải ảnh demo %s: %w", fileName, err)
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return c.responseError("tải ảnh demo "+fileName, status, body)
	}

	return nil
}

func (c *supabaseDemoClient) newRequest(
	ctx context.Context,
	method string,
	path string,
	body io.Reader,
) (*http.Request, error) {
	if c.baseURL == "" || c.serviceKey == "" {
		return nil, fmt.Errorf("thiếu SUPABASE_URL hoặc SUPABASE_SECRET_KEY")
	}

	request, err := http.NewRequestWithContext(
		ctx,
		method,
		c.baseURL+path,
		body,
	)
	if err != nil {
		return nil, fmt.Errorf("tạo request Supabase: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+c.serviceKey)
	request.Header.Set("apikey", c.serviceKey)

	return request, nil
}

func (c *supabaseDemoClient) execute(
	request *http.Request,
) ([]byte, int, error) {
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(
		response.Body,
		maxSupabaseResponseBytes+1,
	))
	if err != nil {
		return nil, response.StatusCode, err
	}
	if len(body) > maxSupabaseResponseBytes {
		return nil, response.StatusCode, fmt.Errorf(
			"phản hồi Supabase vượt quá %d byte",
			maxSupabaseResponseBytes,
		)
	}

	return body, response.StatusCode, nil
}

func (c *supabaseDemoClient) responseError(
	action string,
	status int,
	body []byte,
) error {
	var payload struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	_ = json.Unmarshal(body, &payload)

	message := strings.TrimSpace(payload.Message)
	if message == "" {
		message = strings.TrimSpace(payload.Error)
	}
	if message == "" {
		message = http.StatusText(status)
	}

	return fmt.Errorf("%s: HTTP %d: %s", action, status, message)
}

func isMissingBucketResponse(status int, body []byte) bool {
	if status != http.StatusBadRequest {
		return false
	}

	return strings.Contains(
		strings.ToLower(string(body)),
		"bucket not found",
	)
}
