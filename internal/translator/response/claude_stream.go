// Package response provides StreamTranslator implementations for upstream provider SSE streams.
package response

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/xdotech/gorouter/internal/translator/types"
)

// toolCallState tracks a buffered tool_use block during streaming.
type toolCallState struct {
	Index     int
	ID        string
	Name      string
	ArgsAccum string
}

// ClaudeStreamTranslator translates Claude SSE events to OpenAI SSE format.
type ClaudeStreamTranslator struct {
	model            string
	messageID        string
	usage            types.Usage
	toolCalls        map[int]*toolCallState
	toolCallIndex    int
	serverToolIdx    int
	inThinkingBlock  bool
	thinkingBlockIdx int
	finishReasonSent bool
}

// NewClaudeStreamTranslator creates a translator for Claude → OpenAI SSE conversion.
func NewClaudeStreamTranslator(model string) *ClaudeStreamTranslator {
	return &ClaudeStreamTranslator{
		model:         model,
		toolCalls:     make(map[int]*toolCallState),
		serverToolIdx: -1,
	}
}

// TranslateEvent processes a single data line from the Claude SSE stream.
func (t *ClaudeStreamTranslator) TranslateEvent(line string) ([]types.SSEEvent, error) {
	var chunk map[string]interface{}
	if err := json.Unmarshal([]byte(line), &chunk); err != nil {
		return nil, fmt.Errorf("claude parse: %w", err)
	}

	eventType, _ := chunk["type"].(string)
	var events []types.SSEEvent

	switch eventType {
	case "message_start":
		events = t.handleMessageStart(chunk)
	case "content_block_start":
		events = t.handleContentBlockStart(chunk)
	case "content_block_delta":
		events = t.handleContentBlockDelta(chunk)
	case "content_block_stop":
		events = t.handleContentBlockStop(chunk)
	case "message_delta":
		events = t.handleMessageDelta(chunk)
	case "message_stop":
		events = t.handleMessageStop()
	}

	return events, nil
}

// Flush emits any remaining events. Claude SSE is fully event-driven so nothing to flush.
func (t *ClaudeStreamTranslator) Flush() ([]types.SSEEvent, error) {
	return nil, nil
}

// GetUsage returns the accumulated token usage.
func (t *ClaudeStreamTranslator) GetUsage() *types.Usage {
	return &t.usage
}

func (t *ClaudeStreamTranslator) handleMessageStart(chunk map[string]interface{}) []types.SSEEvent {
	msg, _ := chunk["message"].(map[string]interface{})
	if msg != nil {
		if id, _ := msg["id"].(string); id != "" {
			t.messageID = id
		}
		if model, _ := msg["model"].(string); model != "" {
			t.model = model
		}
	}
	if t.messageID == "" {
		t.messageID = fmt.Sprintf("msg_%d", time.Now().UnixMilli())
	}
	return []types.SSEEvent{t.makeEvent(t.buildOpenAIChunk(map[string]interface{}{"role": "assistant"}, ""))}
}

func (t *ClaudeStreamTranslator) handleContentBlockStart(chunk map[string]interface{}) []types.SSEEvent {
	block, _ := chunk["content_block"].(map[string]interface{})
	if block == nil {
		return nil
	}
	idx := intVal(chunk["index"])
	blockType, _ := block["type"].(string)

	switch blockType {
	case "server_tool_use":
		t.serverToolIdx = idx

	case "thinking":
		t.inThinkingBlock = true
		t.thinkingBlockIdx = idx
		return []types.SSEEvent{t.makeEvent(t.buildOpenAIChunk(map[string]interface{}{"content": "<think>"}, ""))}

	case "tool_use":
		tc := &toolCallState{
			Index: t.toolCallIndex,
			ID:    strVal(block["id"]),
			Name:  strVal(block["name"]),
		}
		t.toolCalls[idx] = tc
		t.toolCallIndex++

		toolCallChunk := map[string]interface{}{
			"tool_calls": []interface{}{
				map[string]interface{}{
					"index": tc.Index,
					"id":    tc.ID,
					"type":  "function",
					"function": map[string]interface{}{
						"name":      tc.Name,
						"arguments": "",
					},
				},
			},
		}
		return []types.SSEEvent{t.makeEvent(t.buildOpenAIChunk(toolCallChunk, ""))}
	}
	return nil
}

func (t *ClaudeStreamTranslator) handleContentBlockDelta(chunk map[string]interface{}) []types.SSEEvent {
	idx := intVal(chunk["index"])
	if idx == t.serverToolIdx {
		return nil
	}
	delta, _ := chunk["delta"].(map[string]interface{})
	if delta == nil {
		return nil
	}
	deltaType, _ := delta["type"].(string)

	switch deltaType {
	case "text_delta":
		text, _ := delta["text"].(string)
		if text == "" {
			return nil
		}
		return []types.SSEEvent{t.makeEvent(t.buildOpenAIChunk(map[string]interface{}{"content": text}, ""))}

	case "thinking_delta":
		thinking, _ := delta["thinking"].(string)
		if thinking == "" {
			return nil
		}
		return []types.SSEEvent{t.makeEvent(t.buildOpenAIChunk(map[string]interface{}{"reasoning_content": thinking}, ""))}

	case "input_json_delta":
		partial, _ := delta["partial_json"].(string)
		tc := t.toolCalls[idx]
		if tc == nil || partial == "" {
			return nil
		}
		tc.ArgsAccum += partial
		toolCallDelta := map[string]interface{}{
			"tool_calls": []interface{}{
				map[string]interface{}{
					"index": tc.Index,
					"id":    tc.ID,
					"function": map[string]interface{}{
						"arguments": partial,
					},
				},
			},
		}
		return []types.SSEEvent{t.makeEvent(t.buildOpenAIChunk(toolCallDelta, ""))}
	}
	return nil
}

func (t *ClaudeStreamTranslator) handleContentBlockStop(chunk map[string]interface{}) []types.SSEEvent {
	idx := intVal(chunk["index"])
	if idx == t.serverToolIdx {
		t.serverToolIdx = -1
		return nil
	}
	if t.inThinkingBlock && idx == t.thinkingBlockIdx {
		t.inThinkingBlock = false
		return []types.SSEEvent{t.makeEvent(t.buildOpenAIChunk(map[string]interface{}{"reasoning_content": ""}, ""))}
	}
	return nil
}

func (t *ClaudeStreamTranslator) handleMessageDelta(chunk map[string]interface{}) []types.SSEEvent {
	if usage, ok := chunk["usage"].(map[string]interface{}); ok {
		input := intVal(usage["input_tokens"])
		output := intVal(usage["output_tokens"])
		cacheRead := intVal(usage["cache_read_input_tokens"])
		cacheCreate := intVal(usage["cache_creation_input_tokens"])
		prompt := input + cacheRead + cacheCreate
		t.usage = types.Usage{
			PromptTokens:     prompt,
			CompletionTokens: output,
			TotalTokens:      prompt + output,
		}
	}

	delta, _ := chunk["delta"].(map[string]interface{})
	if delta == nil {
		return nil
	}
	stopReason, _ := delta["stop_reason"].(string)
	if stopReason == "" {
		return nil
	}
	finishReason := claudeStopToFinishReason(stopReason)

	finalChunk := t.buildOpenAIChunk(map[string]interface{}{}, finishReason)
	if t.usage.TotalTokens > 0 {
		finalChunk["usage"] = map[string]interface{}{
			"prompt_tokens":     t.usage.PromptTokens,
			"completion_tokens": t.usage.CompletionTokens,
			"total_tokens":      t.usage.TotalTokens,
		}
	}
	t.finishReasonSent = true
	return []types.SSEEvent{sseData(finalChunk)}
}

func (t *ClaudeStreamTranslator) handleMessageStop() []types.SSEEvent {
	if t.finishReasonSent {
		return nil
	}
	finishReason := "stop"
	if len(t.toolCalls) > 0 {
		finishReason = "tool_calls"
	}
	return []types.SSEEvent{t.makeEvent(t.buildOpenAIChunk(map[string]interface{}{}, finishReason))}
}

func (t *ClaudeStreamTranslator) makeEvent(chunk map[string]interface{}) types.SSEEvent {
	return sseData(chunk)
}

func (t *ClaudeStreamTranslator) buildOpenAIChunk(delta interface{}, finishReason string) map[string]interface{} {
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

func sseData(v map[string]interface{}) types.SSEEvent {
	b, _ := json.Marshal(v)
	return types.SSEEvent{Data: string(b)}
}

func claudeStopToFinishReason(reason string) string {
	switch reason {
	case "end_turn":
		return "stop"
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	case "stop_sequence":
		return "stop"
	default:
		return "stop"
	}
}

func strVal(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func intVal(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}
