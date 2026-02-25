package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	geminiBaseURL    = "https://cloudcode-pa.googleapis.com/v1/projects/%s/locations/us-central1/publishers/google/models/%s:streamGenerateContent?alt=sse"
	googleTokenURL   = "https://oauth2.googleapis.com/token"
	geminiClientID   = "681254865684-mb2p5jgo4qiutapls8r.apps.googleusercontent.com"
)

type geminiCLIExecutor struct {
	client *http.Client
}

func newGeminiCLIExecutor(client *http.Client) *geminiCLIExecutor {
	return &geminiCLIExecutor{client: client}
}

func (e *geminiCLIExecutor) Execute(ctx context.Context, _, model string, bodyBytes []byte, creds Credentials) (*ExecuteResult, error) {
	endpoint := fmt.Sprintf(geminiBaseURL, creds.ProjectID, model)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("gemini-cli executor: build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini-cli executor: send request to %s: %w", endpoint, err)
	}

	return &ExecuteResult{
		Body:       resp.Body,
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		IsStream:   isSSE(resp),
	}, nil
}

func (e *geminiCLIExecutor) SupportsRefresh() bool { return true }

func (e *geminiCLIExecutor) RefreshCredentials(ctx context.Context, creds Credentials) (*Credentials, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", creds.RefreshToken)
	form.Set("client_id", geminiClientID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleTokenURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("gemini-cli refresh: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini-cli refresh: send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gemini-cli refresh: upstream returned %d: %s", resp.StatusCode, body)
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("gemini-cli refresh: decode response: %w", err)
	}

	updated := creds
	updated.AccessToken = tokenResp.AccessToken
	if tokenResp.RefreshToken != "" {
		updated.RefreshToken = tokenResp.RefreshToken
	}
	return &updated, nil
}
