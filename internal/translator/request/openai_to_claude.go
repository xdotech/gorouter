// Package request provides translators that convert request bodies between AI provider formats.
package request

import (
	"encoding/json"
)

const defaultMaxTokens = 8096

// OpenAIToClaude translates an OpenAI chat completions body to the Anthropic Messages API format.
// It handles message role mapping, tool conversion, and generation parameters.
func OpenAIToClaude(body map[string]interface{}) (map[string]interface{}, error) {
	result := map[string]interface{}{}

	// Model
	if model, ok := body["model"].(string); ok {
		result["model"] = model
	}

	// Stream
	if stream, ok := body["stream"]; ok {
		result["stream"] = stream
	}

	// max_tokens — default if not provided
	if mt, ok := toInt(body["max_tokens"]); ok && mt > 0 {
		result["max_tokens"] = mt
	} else {
		result["max_tokens"] = defaultMaxTokens
	}

	// temperature
	if temp, ok := body["temperature"]; ok {
		result["temperature"] = temp
	}

	// top_p
	if tp, ok := body["top_p"]; ok {
		result["top_p"] = tp
	}

	// Build messages and system
	system, messages := convertMessages(body)
	if len(system) > 0 {
		result["system"] = system
	}
	result["messages"] = messages

	// Tools
	if tools, ok := body["tools"].([]interface{}); ok && len(tools) > 0 {
		result["tools"] = convertToolsToClaude(tools)
	}

	// tool_choice
	if tc, ok := body["tool_choice"]; ok && tc != nil {
		result["tool_choice"] = convertToolChoice(tc)
	}

	return result, nil
}

// convertMessages splits OpenAI messages into a Claude system field and messages array.
func convertMessages(body map[string]interface{}) ([]interface{}, []interface{}) {
	msgs, _ := body["messages"].([]interface{})
	var systemParts []string
	var out []interface{}

	for _, m := range msgs {
		msg, ok := m.(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := msg["role"].(string)

		switch role {
		case "system":
			text := extractText(msg["content"])
			if text != "" {
				systemParts = append(systemParts, text)
			}

		case "user":
			blocks := userContentToBlocks(msg["content"])
			if len(blocks) > 0 {
				out = append(out, map[string]interface{}{"role": "user", "content": blocks})
			}

		case "tool":
			block := map[string]interface{}{
				"type":        "tool_result",
				"tool_use_id": strVal(msg["tool_call_id"]),
				"content":     strVal(msg["content"]),
			}
			out = append(out, map[string]interface{}{"role": "user", "content": []interface{}{block}})

		case "assistant":
			blocks := assistantContentToBlocks(msg)
			if len(blocks) > 0 {
				out = append(out, map[string]interface{}{"role": "assistant", "content": blocks})
			}
		}
	}

	var systemBlocks []interface{}
	for _, s := range systemParts {
		systemBlocks = append(systemBlocks, map[string]interface{}{"type": "text", "text": s})
	}

	return systemBlocks, out
}

// userContentToBlocks converts OpenAI user message content to Claude content blocks.
func userContentToBlocks(content interface{}) []interface{} {
	var blocks []interface{}

	switch v := content.(type) {
	case string:
		if v != "" {
			blocks = append(blocks, map[string]interface{}{"type": "text", "text": v})
		}
	case []interface{}:
		for _, p := range v {
			part, ok := p.(map[string]interface{})
			if !ok {
				continue
			}
			partType, _ := part["type"].(string)
			switch partType {
			case "text":
				if text, _ := part["text"].(string); text != "" {
					blocks = append(blocks, map[string]interface{}{"type": "text", "text": text})
				}
			case "image_url":
				if block := imageURLToBlock(part); block != nil {
					blocks = append(blocks, block)
				}
			case "tool_result":
				blocks = append(blocks, map[string]interface{}{
					"type":        "tool_result",
					"tool_use_id": strVal(part["tool_use_id"]),
					"content":     strVal(part["content"]),
				})
			}
		}
	}
	return blocks
}

// assistantContentToBlocks converts an OpenAI assistant message to Claude content blocks.
func assistantContentToBlocks(msg map[string]interface{}) []interface{} {
	var blocks []interface{}

	// Text content
	if text := extractText(msg["content"]); text != "" {
		blocks = append(blocks, map[string]interface{}{"type": "text", "text": text})
	}

	// Tool calls
	if tcs, ok := msg["tool_calls"].([]interface{}); ok {
		for _, t := range tcs {
			tc, ok := t.(map[string]interface{})
			if !ok {
				continue
			}
			fn, _ := tc["function"].(map[string]interface{})
			if fn == nil {
				continue
			}
			input := parseJSONArg(strVal(fn["arguments"]))
			blocks = append(blocks, map[string]interface{}{
				"type":  "tool_use",
				"id":    strVal(tc["id"]),
				"name":  strVal(fn["name"]),
				"input": input,
			})
		}
	}
	return blocks
}

// convertToolsToClaude converts OpenAI tools array to Claude tools array.
func convertToolsToClaude(tools []interface{}) []interface{} {
	var out []interface{}
	for _, t := range tools {
		tool, ok := t.(map[string]interface{})
		if !ok {
			continue
		}
		toolType, _ := tool["type"].(string)
		var fn map[string]interface{}
		if toolType == "function" {
			fn, _ = tool["function"].(map[string]interface{})
		} else if tool["name"] != nil {
			fn = tool
		}
		if fn == nil {
			continue
		}
		schema := fn["parameters"]
		if schema == nil {
			schema = fn["input_schema"]
		}
		if schema == nil {
			schema = map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
		}
		out = append(out, map[string]interface{}{
			"name":         strVal(fn["name"]),
			"description":  strVal(fn["description"]),
			"input_schema": schema,
		})
	}
	return out
}

// convertToolChoice maps OpenAI tool_choice to Claude tool_choice.
func convertToolChoice(tc interface{}) interface{} {
	switch v := tc.(type) {
	case string:
		switch v {
		case "required":
			return map[string]interface{}{"type": "any"}
		case "none", "auto":
			return map[string]interface{}{"type": "auto"}
		}
	case map[string]interface{}:
		if fn, ok := v["function"].(map[string]interface{}); ok {
			return map[string]interface{}{"type": "tool", "name": strVal(fn["name"])}
		}
		if v["type"] != nil {
			return v
		}
	}
	return map[string]interface{}{"type": "auto"}
}

// imageURLToBlock converts an OpenAI image_url part to a Claude image block.
func imageURLToBlock(part map[string]interface{}) map[string]interface{} {
	imgURL, _ := part["image_url"].(map[string]interface{})
	if imgURL == nil {
		return nil
	}
	url, _ := imgURL["url"].(string)
	if url == "" {
		return nil
	}
	// data URI: data:image/png;base64,<data>
	if len(url) > 5 && url[:5] == "data:" {
		semi := indexOf(url, ';')
		comma := indexOf(url, ',')
		if semi > 0 && comma > semi {
			mediaType := url[5:semi]
			data := url[comma+1:]
			return map[string]interface{}{
				"type": "image",
				"source": map[string]interface{}{
					"type":       "base64",
					"media_type": mediaType,
					"data":       data,
				},
			}
		}
	}
	return nil
}

// --- helpers ---

func extractText(content interface{}) string {
	switch v := content.(type) {
	case string:
		return v
	case []interface{}:
		var out string
		for _, p := range v {
			if part, ok := p.(map[string]interface{}); ok {
				if part["type"] == "text" {
					out += strVal(part["text"])
				}
			}
		}
		return out
	}
	return ""
}

func strVal(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func toInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	case json.Number:
		i, err := n.Int64()
		return int(i), err == nil
	}
	return 0, false
}

func parseJSONArg(s string) interface{} {
	if s == "" {
		return map[string]interface{}{}
	}
	var out interface{}
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return s
	}
	return out
}

func indexOf(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}
