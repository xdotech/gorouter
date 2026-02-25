package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	githubCopilotTokenURL       = "https://api.github.com/copilot_internal/v2/token"
	githubCopilotCompletionsURL = "https://api.githubcopilot.com/chat/completions"
)

type githubExecutor struct {
	client *http.Client
}

func newGitHubExecutor(client *http.Client) *githubExecutor {
	return &githubExecutor{client: client}
}

func (e *githubExecutor) Execute(ctx context.Context, _, _ string, bodyBytes []byte, creds Credentials) (*ExecuteResult, error) {
	copilotToken, err := e.resolveCopilotToken(ctx, creds)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, githubCopilotCompletionsURL,
		bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("github executor: build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+copilotToken)
	req.Header.Set("Editor-Version", "vscode/1.95.0")
	req.Header.Set("Copilot-Integration-Id", "vscode-chat")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github executor: send request: %w", err)
	}

	return &ExecuteResult{
		Body:       resp.Body,
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		IsStream:   isSSE(resp),
	}, nil
}

func (e *githubExecutor) SupportsRefresh() bool { return false }

func (e *githubExecutor) RefreshCredentials(_ context.Context, _ Credentials) (*Credentials, error) {
	return nil, fmt.Errorf("github executor does not support token refresh")
}

// resolveCopilotToken returns the cached CopilotToken from creds, or exchanges
// the GitHub AccessToken for a fresh Copilot token.
func (e *githubExecutor) resolveCopilotToken(ctx context.Context, creds Credentials) (string, error) {
	if creds.CopilotToken != "" {
		return creds.CopilotToken, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubCopilotTokenURL, nil)
	if err != nil {
		return "", fmt.Errorf("github executor: build token request: %w", err)
	}
	req.Header.Set("Authorization", "token "+creds.AccessToken)

	resp, err := e.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("github executor: fetch copilot token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("github executor: copilot token exchange returned %d: %s", resp.StatusCode, body)
	}

	var tokenResp struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("github executor: decode copilot token: %w", err)
	}
	if tokenResp.Token == "" {
		return "", fmt.Errorf("github executor: empty copilot token in response")
	}
	return tokenResp.Token, nil
}
