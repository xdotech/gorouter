package response

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/xdotech/gorouter/internal/translator/types"
)

// GeminiStreamTranslator translates Gemini SSE events to OpenAI SSE format.
type GeminiStreamTranslator struct {
	model         string
	messageID     string
	usage         types.Usage
	functionIndex int
	initialized   bool
}

// NewGeminiStreamTranslator creates a translator for Gemini → OpenAI SSE conversion.
func NewGeminiStreamTranslator(model string) *GeminiStreamTranslator {
	return &GeminiStreamTranslator{model: model}
}

// TranslateEvent processes a single data line from the Gemini SSE stream.
func (t *GeminiStreamTranslator) TranslateEvent(line string) ([]types.SSEEvent, error) {
	var chunk map[string]interface{}
	if err := json.Unmarshal([]byte(line), &chunk); err != nil {
		return nil, fmt.Errorf("gemini parse: %w", err)
	}

	// Gemini CLI wraps response under "response" key
	response, _ := chunk["response"].(map[string]interface{})
	if response == nil {
		response = chunk
	}

	candidates, _ := response["candidates"].([]interface{})
	if len(candidates) == 0 {
		return nil, nil
	}
	candidate, _ := candidates[0].(map[string]interface{})
	if candidate == nil {
		return nil, nil
	}

	var events []types.SSEEvent

	// Initialize once with role event
	if !t.initialized {
		t.initialized = true
		respID, _ := response["responseId"].(string)
		if respID == "" {
			respID = fmt.Sprintf("msg_%d", time.Now().UnixMilli())
		}
		t.messageID = respID
		if mv, _ := response["modelVersion"].(string); mv != "" {
			t.model = mv
		}
		events = append(events, sseData(t.buildChunk(map[string]interface{}{"role": "assistant"}, "")))
	}

	// Process content parts
	content, _ := candidate["content"].(map[string]interface{})
	if content != nil {
		parts, _ := content["parts"].([]interface{})
		for _, p := range parts {
			part, ok := p.(map[string]interface{})
			if !ok {
				continue
			}
			events = append(events, t.processPart(part)...)
		}
	}

	// Usage metadata
	if meta := getUsageMeta(response, chunk); meta != nil {
		t.extractUsage(meta)
	}

	// Finish reason
	if fr, _ := candidate["finishReason"].(string); fr != "" {
		finalChunk := t.buildChunk(map[string]interface{}{}, normalizeFinishReason(fr))
		if t.usage.TotalTokens > 0 {
			finalChunk["usage"] = map[string]interface{}{
				"prompt_tokens":     t.usage.PromptTokens,
				"completion_tokens": t.usage.CompletionTokens,
				"total_tokens":      t.usage.TotalTokens,
			}
		}
		events = append(events, sseData(finalChunk))
	}

	return events, nil
}

// Flush is a no-op for Gemini since all data arrives in discrete chunks.
func (t *GeminiStreamTranslator) Flush() ([]types.SSEEvent, error) {
	return nil, nil
}

// GetUsage returns accumulated token usage.
func (t *GeminiStreamTranslator) GetUsage() *types.Usage {
	return &t.usage
}

func (t *GeminiStreamTranslator) processPart(part map[string]interface{}) []types.SSEEvent {
	var events []types.SSEEvent

	hasThoughtSig := part["thoughtSignature"] != nil || part["thought_signature"] != nil
	isThought, _ := part["thought"].(bool)

	if hasThoughtSig {
		if text, _ := part["text"].(string); text != "" {
			delta := map[string]interface{}{}
			if isThought {
				delta["reasoning_content"] = text
			} else {
				delta["content"] = text
			}
			events = append(events, sseData(t.buildChunk(delta, "")))
		}
		if fc, ok := part["functionCall"].(map[string]interface{}); ok {
			events = append(events, t.functionCallEvent(fc))
		}
		return events
	}

	// Plain text
	if text, _ := part["text"].(string); text != "" {
		events = append(events, sseData(t.buildChunk(map[string]interface{}{"content": text}, "")))
	}

	// Function call
	if fc, ok := part["functionCall"].(map[string]interface{}); ok {
		events = append(events, t.functionCallEvent(fc))
	}

	// Inline data (images)
	if inlineData := getInlineData(part); inlineData != nil {
		if data, _ := inlineData["data"].(string); data != "" {
			mimeType, _ := inlineData["mimeType"].(string)
			if mimeType == "" {
				mimeType, _ = inlineData["mime_type"].(string)
			}
			if mimeType == "" {
				mimeType = "image/png"
			}
			imgDelta := map[string]interface{}{
				"images": []interface{}{
					map[string]interface{}{
						"type":      "image_url",
						"image_url": map[string]interface{}{"url": fmt.Sprintf("data:%s;base64,%s", mimeType, data)},
					},
				},
			}
			events = append(events, sseData(t.buildChunk(imgDelta, "")))
		}
	}

	return events
}

func (t *GeminiStreamTranslator) functionCallEvent(fc map[string]interface{}) types.SSEEvent {
	name, _ := fc["name"].(string)
	args := fc["args"]
	if args == nil {
		args = map[string]interface{}{}
	}
	argsJSON, _ := json.Marshal(args)
	idx := t.functionIndex
	t.functionIndex++

	toolCall := map[string]interface{}{
		"index": idx,
		"id":    fmt.Sprintf("%s-%d-%d", name, time.Now().UnixMilli(), idx),
		"type":  "function",
		"function": map[string]interface{}{
			"name":      name,
			"arguments": string(argsJSON),
		},
	}
	return sseData(t.buildChunk(map[string]interface{}{"tool_calls": []interface{}{toolCall}}, ""))
}

func (t *GeminiStreamTranslator) buildChunk(delta interface{}, finishReason string) map[string]interface{} {
	var fr interface{}
	if finishReason != "" {
		fr = finishReason
	}
	return map[string]interface{}{
		"id":      fmt.Sprintf("chatcmpl-%s", t.messageID),
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   t.model,
		"choices": []interface{}{
			map[string]interface{}{
				"index":         0,
				"delta":         delta,
				"finish_reason": fr,
			},
		},
	}
}

func (t *GeminiStreamTranslator) extractUsage(meta map[string]interface{}) {
	promptRaw := intVal(meta["promptTokenCount"])
	candidates := intVal(meta["candidatesTokenCount"])
	thoughts := intVal(meta["thoughtsTokenCount"])
	total := intVal(meta["totalTokenCount"])

	if candidates == 0 && total > 0 {
		candidates = total - promptRaw - thoughts
		if candidates < 0 {
			candidates = 0
		}
	}
	t.usage = types.Usage{
		PromptTokens:     promptRaw,
		CompletionTokens: candidates + thoughts,
		TotalTokens:      total,
	}
}

func getUsageMeta(response, chunk map[string]interface{}) map[string]interface{} {
	if m, ok := response["usageMetadata"].(map[string]interface{}); ok {
		return m
	}
	if m, ok := chunk["usageMetadata"].(map[string]interface{}); ok {
		return m
	}
	return nil
}

func getInlineData(part map[string]interface{}) map[string]interface{} {
	if d, ok := part["inlineData"].(map[string]interface{}); ok {
		return d
	}
	if d, ok := part["inline_data"].(map[string]interface{}); ok {
		return d
	}
	return nil
}

func normalizeFinishReason(reason string) string {
	switch reason {
	case "STOP", "stop":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY":
		return "content_filter"
	default:
		return "stop"
	}
}
