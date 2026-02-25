package executor

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
)

// providerBaseURLs maps short provider names to their OpenAI-compatible base URLs.
var providerBaseURLs = map[string]string{
	"glm":        "https://open.bigmodel.cn/api/paas/v4",
	"minimax":    "https://api.minimaxi.chat/v1",
	"kimi":       "https://api.moonshot.cn/v1",
	"if":         "https://iflow.ai/api/v1",
	"qw":         "https://dashscope.aliyuncs.com/compatible-mode/v1",
	"openai":     "https://api.openai.com/v1",
	"anthropic":  "https://api.anthropic.com/v1",
	"openrouter": "https://openrouter.ai/api/v1",
}

type defaultExecutor struct {
	client *http.Client
}

func newDefaultExecutor(client *http.Client) *defaultExecutor {
	return &defaultExecutor{client: client}
}

func (e *defaultExecutor) Execute(ctx context.Context, provider, model string, bodyBytes []byte, creds Credentials) (*ExecuteResult, error) {
	baseURL := e.resolveBaseURL(provider, creds)

	// Anthropic uses /messages; everyone else uses /chat/completions.
	endpoint := "/chat/completions"
	if provider == "anthropic" {
		endpoint = "/messages"
	}

	url := baseURL + endpoint

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("default executor: build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Prefer AccessToken over APIKey (some providers issue OAuth tokens).
	token := creds.APIKey
	if creds.AccessToken != "" {
		token = creds.AccessToken
	}
	req.Header.Set("Authorization", "Bearer "+token)

	if provider == "anthropic" {
		req.Header.Set("anthropic-version", "2023-06-01")
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("default executor: send request to %s: %w", url, err)
	}

	return &ExecuteResult{
		Body:       resp.Body,
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		IsStream:   isSSE(resp),
	}, nil
}

func (e *defaultExecutor) RefreshCredentials(_ context.Context, _ Credentials) (*Credentials, error) {
	return nil, fmt.Errorf("default executor does not support token refresh")
}

func (e *defaultExecutor) SupportsRefresh() bool { return false }

// resolveBaseURL picks the base URL for the provider, with override from connection data.
func (e *defaultExecutor) resolveBaseURL(provider string, creds Credentials) string {
	if creds.ProviderSpecificData != nil {
		if v, ok := creds.ProviderSpecificData["baseUrl"]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	if url, ok := providerBaseURLs[provider]; ok {
		return url
	}
	// Unknown provider: fall back to OpenAI-compatible endpoint.
	return providerBaseURLs["openai"]
}

// isSSE reports whether the response looks like a server-sent event stream.
func isSSE(resp *http.Response) bool {
	ct := resp.Header.Get("Content-Type")
	return len(ct) >= 17 && ct[:17] == "text/event-stream"
}
