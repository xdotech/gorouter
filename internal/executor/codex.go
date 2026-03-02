package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

const (
	codexResponsesURL        = "https://chatgpt.com/backend-api/codex/responses"
	codexDefaultInstructions = "You are a helpful AI coding assistant. Follow the user's instructions carefully. Write clean, readable code with appropriate comments. When asked to modify code, explain what changes you made and why."
)

type codexExecutor struct {
	client *http.Client
}

func newCodexExecutor(client *http.Client) *codexExecutor {
	return &codexExecutor{client: client}
}

func (e *codexExecutor) Execute(ctx context.Context, _, model string, bodyBytes []byte, creds Credentials) (*ExecuteResult, error) {
	// Parse the body to add Codex-specific params
	var body map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		return nil, fmt.Errorf("codex executor: parse body: %w", err)
	}

	// Convert Chat Completions format (messages) → Responses API format (input)
	if msgs, ok := body["messages"].([]interface{}); ok {
		var input []interface{}
		for _, m := range msgs {
			msg, ok := m.(map[string]interface{})
			if !ok {
				continue
			}
			role, _ := msg["role"].(string)

			// System messages become instructions
			if role == "system" {
				if text, ok := msg["content"].(string); ok {
					body["instructions"] = text
				}
				continue
			}

			// Convert content to Responses API content block format
			// User messages use "input_text", assistant messages use "output_text"
			var contentBlocks []interface{}
			contentType := "input_text"
			if role == "assistant" {
				contentType = "output_text"
			}
			switch c := msg["content"].(type) {
			case string:
				contentBlocks = []interface{}{
					map[string]interface{}{"type": contentType, "text": c},
				}
			case []interface{}:
				// Already structured content blocks, pass through
				contentBlocks = c
			}

			input = append(input, map[string]interface{}{
				"type":    "message",
				"role":    role,
				"content": contentBlocks,
			})
		}
		delete(body, "messages")
		body["input"] = input
	}

	// Ensure input is present and non-empty
	if input, ok := body["input"]; !ok || input == nil {
		body["input"] = []interface{}{
			map[string]interface{}{
				"type": "message",
				"role": "user",
				"content": []interface{}{
					map[string]interface{}{"type": "input_text", "text": "..."},
				},
			},
		}
	}

	// Inject default instructions if missing (Codex API requires instructions)
	if instructions, ok := body["instructions"].(string); !ok || instructions == "" {
		body["instructions"] = codexDefaultInstructions
	}

	// Force stream and store
	body["stream"] = true
	body["store"] = false

	// Set up reasoning (effort + summary)
	if _, ok := body["reasoning"]; !ok {
		effort := "medium"
		if re, ok := body["reasoning_effort"].(string); ok && re != "" {
			effort = re
		}
		body["reasoning"] = map[string]interface{}{
			"effort":  effort,
			"summary": "auto",
		}
	}
	delete(body, "reasoning_effort")

	// Include encrypted reasoning content for reasoning models
	if reasoning, ok := body["reasoning"].(map[string]interface{}); ok {
		if effort, _ := reasoning["effort"].(string); effort != "" && effort != "none" {
			body["include"] = []string{"reasoning.encrypted_content"}
		}
	}

	// Remove unsupported parameters
	for _, key := range []string{
		"temperature", "top_p", "frequency_penalty", "presence_penalty",
		"logprobs", "top_logprobs", "n", "seed", "max_tokens",
		"user", "prompt_cache_retention", "metadata", "stream_options",
		"safety_identifier",
	} {
		delete(body, key)
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("codex executor: marshal body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, codexResponsesURL, bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("codex executor: build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	req.Header.Set("originator", "codex-cli")
	req.Header.Set("User-Agent", "codex-cli/1.0.18 (macOS; arm64)")
	req.Header.Set("session_id", strconv.FormatInt(time.Now().UnixMilli(), 10)+"-"+randomString(9))

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("codex executor: send request: %w", err)
	}

	return &ExecuteResult{
		Body:       resp.Body,
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		IsStream:   true, // Codex API always streams but returns Content-Type: application/json
	}, nil
}

func (e *codexExecutor) SupportsRefresh() bool { return true }

func (e *codexExecutor) RefreshCredentials(ctx context.Context, creds Credentials) (*Credentials, error) {
	form := "grant_type=refresh_token&refresh_token=" + creds.RefreshToken +
		"&client_id=app_EMoamEEZ73f0CkXaXp7hrann"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://auth.openai.com/oauth/token",
		bytes.NewReader([]byte(form)))
	if err != nil {
		return nil, fmt.Errorf("codex refresh: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("codex refresh: send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("codex refresh: upstream returned %d", resp.StatusCode)
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("codex refresh: decode response: %w", err)
	}

	updated := creds
	updated.AccessToken = tokenResp.AccessToken
	if tokenResp.RefreshToken != "" {
		updated.RefreshToken = tokenResp.RefreshToken
	}
	return &updated, nil
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
