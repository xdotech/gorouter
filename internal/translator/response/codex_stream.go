package response

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/xdotech/gorouter/internal/translator/types"
)

// CodexStreamTranslator translates OpenAI Responses API SSE events to Chat Completions SSE format.
type CodexStreamTranslator struct {
	model     string
	messageID string
	usage     types.Usage
	created   int64
}

// NewCodexStreamTranslator creates a translator for Codex Responses API → Chat Completions SSE.
func NewCodexStreamTranslator(model string) *CodexStreamTranslator {
	return &CodexStreamTranslator{
		model:   model,
		created: time.Now().Unix(),
	}
}

// TranslateEvent processes a single Responses API SSE data line into Chat Completions chunks.
func (t *CodexStreamTranslator) TranslateEvent(line string) ([]types.SSEEvent, error) {
	var chunk map[string]interface{}
	if err := json.Unmarshal([]byte(line), &chunk); err != nil {
		return nil, fmt.Errorf("codex stream parse: %w", err)
	}

	eventType, _ := chunk["type"].(string)

	switch eventType {
	case "response.created":
		// Extract response ID and model
		if resp, ok := chunk["response"].(map[string]interface{}); ok {
			if id, ok := resp["id"].(string); ok {
				t.messageID = id
			}
			if m, ok := resp["model"].(string); ok {
				t.model = m
			}
		}
		// Emit initial role chunk
		return []types.SSEEvent{t.buildChunk(map[string]interface{}{
			"role":    "assistant",
			"content": "",
		}, nil)}, nil

	case "response.output_text.delta":
		// Text content delta
		delta, _ := chunk["delta"].(string)
		if delta != "" {
			return []types.SSEEvent{t.buildChunk(map[string]interface{}{
				"content": delta,
			}, nil)}, nil
		}

	case "response.output_text.done":
		// Text complete — no action needed, we'll get response.completed

	case "response.completed":
		// Extract usage from completed response
		if resp, ok := chunk["response"].(map[string]interface{}); ok {
			if u, ok := resp["usage"].(map[string]interface{}); ok {
				t.usage.PromptTokens = jsonInt(u, "input_tokens")
				t.usage.CompletionTokens = jsonInt(u, "output_tokens")
				t.usage.TotalTokens = t.usage.PromptTokens + t.usage.CompletionTokens
			}
		}
		// Emit finish chunk
		finish := "stop"
		return []types.SSEEvent{t.buildChunk(map[string]interface{}{}, &finish)}, nil

	case "response.failed":
		// Error in response
		errMsg := "unknown codex error"
		if resp, ok := chunk["response"].(map[string]interface{}); ok {
			if e, ok := resp["error"].(map[string]interface{}); ok {
				if msg, ok := e["message"].(string); ok {
					errMsg = msg
				}
			}
		}
		return []types.SSEEvent{t.buildChunk(map[string]interface{}{
			"content": "[Error: " + errMsg + "]",
		}, nil)}, nil
	}

	return nil, nil
}

// Flush emits any remaining buffered events.
func (t *CodexStreamTranslator) Flush() ([]types.SSEEvent, error) {
	return nil, nil
}

// GetUsage returns accumulated token usage.
func (t *CodexStreamTranslator) GetUsage() *types.Usage {
	return &t.usage
}

// buildChunk creates a Chat Completions SSE chunk.
func (t *CodexStreamTranslator) buildChunk(delta map[string]interface{}, finishReason *string) types.SSEEvent {
	choice := map[string]interface{}{
		"index": 0,
		"delta": delta,
	}
	if finishReason != nil {
		choice["finish_reason"] = *finishReason
	}

	chunk := map[string]interface{}{
		"id":      "chatcmpl-" + t.messageID,
		"object":  "chat.completion.chunk",
		"created": t.created,
		"model":   t.model,
		"choices": []interface{}{choice},
	}

	data, _ := json.Marshal(chunk)
	return types.SSEEvent{Data: string(data)}
}

func jsonInt(m map[string]interface{}, key string) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return 0
}
