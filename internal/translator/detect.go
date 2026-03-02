package translator

// DetectSourceFormat inspects a decoded request body and returns the format constant.
// Detection priority: gemini (has "contents") > openai-responses (has "input" array) >
// claude (has "messages" with content_block-style parts) > openai (default).
func DetectSourceFormat(body map[string]interface{}) string {
	if body == nil {
		return FormatOpenAI
	}

	// Gemini: uses "contents" array instead of "messages"
	if _, ok := body["contents"]; ok {
		return FormatGemini
	}

	// OpenAI Responses API: uses "input" as array
	if input, ok := body["input"]; ok {
		if _, isSlice := input.([]interface{}); isSlice {
			return FormatOpenAIResp
		}
	}

	// Claude: has "messages" and looks like Anthropic Messages API
	// Heuristic: no "model" field that starts with "gpt-", and either has
	// a top-level "system" field or messages with content blocks.
	if msgs, ok := body["messages"]; ok {
		if msgSlice, ok := msgs.([]interface{}); ok && len(msgSlice) > 0 {
			if isClaudeStyle(body, msgSlice) {
				return FormatClaude
			}
		}
	}

	return FormatOpenAI
}

// isClaudeStyle returns true when the body looks like Anthropic Messages API.
// Signals: top-level "system" array, content blocks (type/text objects), or
// no "model" prefix that is clearly openai ("gpt-", "o1", "o3").
func isClaudeStyle(body map[string]interface{}, msgs []interface{}) bool {
	// If "system" is an array of content blocks, it is Claude format.
	if sys, ok := body["system"]; ok {
		if _, isSlice := sys.([]interface{}); isSlice {
			return true
		}
	}

	// Check first message for Claude-style content blocks.
	if first, ok := msgs[0].(map[string]interface{}); ok {
		if content, ok := first["content"]; ok {
			if parts, ok := content.([]interface{}); ok && len(parts) > 0 {
				if part, ok := parts[0].(map[string]interface{}); ok {
					if t, ok := part["type"].(string); ok {
						switch t {
						case "text", "image", "tool_use", "tool_result":
							return true
						}
					}
				}
			}
		}
	}

	// If model field is clearly an OpenAI model, not Claude.
	if model, ok := body["model"].(string); ok {
		if len(model) >= 3 {
			prefix := model[:3]
			if prefix == "gpt" || prefix == "o1-" || prefix == "o3-" {
				return false
			}
		}
	}

	return false
}

// GetTargetFormat maps a provider identifier to the target format constant.
func GetTargetFormat(provider string) string {
	switch provider {
	case "cc", "kr", "anthropic", "claude-code", "kiro":
		return FormatClaude
	case "gc", "gemini-cli":
		return FormatGeminiCLI
	case "antigravity":
		return FormatAntigravity
	case "cursor":
		return FormatCursor
	case "cx", "codex":
		return FormatOpenAIResp
	default:
		return FormatOpenAI
	}
}
