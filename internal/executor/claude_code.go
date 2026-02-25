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
	claudeMessagesURL = "https://api.anthropic.com/v1/messages"
	claudeTokenURL    = "https://claude.ai/oauth/token"
)

type claudeCodeExecutor struct {
	client *http.Client
}

func newClaudeCodeExecutor(client *http.Client) *claudeCodeExecutor {
	return &claudeCodeExecutor{client: client}
}

func (e *claudeCodeExecutor) Execute(ctx context.Context, _, _ string, bodyBytes []byte, creds Credentials) (*ExecuteResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, claudeMessagesURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("claude-code executor: build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("anthropic-beta", "interleaved-thinking-2025-05-14")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("claude-code executor: send request: %w", err)
	}

	return &ExecuteResult{
		Body:       resp.Body,
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		IsStream:   isSSE(resp),
	}, nil
}

func (e *claudeCodeExecutor) SupportsRefresh() bool { return true }

func (e *claudeCodeExecutor) RefreshCredentials(ctx context.Context, creds Credentials) (*Credentials, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", creds.RefreshToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, claudeTokenURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("claude-code refresh: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("claude-code refresh: send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("claude-code refresh: upstream returned %d: %s", resp.StatusCode, body)
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("claude-code refresh: decode response: %w", err)
	}

	updated := creds
	updated.AccessToken = tokenResp.AccessToken
	if tokenResp.RefreshToken != "" {
		updated.RefreshToken = tokenResp.RefreshToken
	}
	return &updated, nil
}
