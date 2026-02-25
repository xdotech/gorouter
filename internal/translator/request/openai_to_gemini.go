package request

import "encoding/json"

// defaultSafetySettings disables all Gemini safety filters for proxy usage.
var defaultSafetySettings = []interface{}{
	map[string]interface{}{"category": "HARM_CATEGORY_HARASSMENT", "threshold": "BLOCK_NONE"},
	map[string]interface{}{"category": "HARM_CATEGORY_HATE_SPEECH", "threshold": "BLOCK_NONE"},
	map[string]interface{}{"category": "HARM_CATEGORY_SEXUALLY_EXPLICIT", "threshold": "BLOCK_NONE"},
	map[string]interface{}{"category": "HARM_CATEGORY_DANGEROUS_CONTENT", "threshold": "BLOCK_NONE"},
}

// OpenAIToGemini translates an OpenAI chat completions body to Gemini generateContent format.
func OpenAIToGemini(body map[string]interface{}) (map[string]interface{}, error) {
	result := map[string]interface{}{
		"contents":       []interface{}{},
		"generationConfig": map[string]interface{}{},
		"safetySettings": defaultSafetySettings,
	}

	genConfig := result["generationConfig"].(map[string]interface{})

	// Generation config fields
	if temp, ok := body["temperature"]; ok {
		genConfig["temperature"] = temp
	}
	if tp, ok := body["top_p"]; ok {
		genConfig["topP"] = tp
	}
	if tk, ok := body["top_k"]; ok {
		genConfig["topK"] = tk
	}
	if mt, ok := toInt(body["max_tokens"]); ok && mt > 0 {
		genConfig["maxOutputTokens"] = mt
	}

	// Build tool_call_id → name lookup from assistant messages
	tcID2Name := buildToolCallIDMap(body)

	// Build tool response cache
	toolResponses := buildToolResponseCache(body)

	// Convert messages → contents + systemInstruction
	contents, systemInstruction := convertMessagesToGemini(body, tcID2Name, toolResponses)
	result["contents"] = contents
	if systemInstruction != nil {
		result["systemInstruction"] = systemInstruction
	}

	// Convert tools
	if tools, ok := body["tools"].([]interface{}); ok && len(tools) > 0 {
		if decls := toolsToFunctionDeclarations(tools); len(decls) > 0 {
			result["tools"] = []interface{}{
				map[string]interface{}{"functionDeclarations": decls},
			}
		}
	}

	return result, nil
}

// buildToolCallIDMap creates a map of tool call IDs to function names from assistant messages.
func buildToolCallIDMap(body map[string]interface{}) map[string]string {
	m := map[string]string{}
	msgs, _ := body["messages"].([]interface{})
	for _, raw := range msgs {
		msg, ok := raw.(map[string]interface{})
		if !ok || msg["role"] != "assistant" {
			continue
		}
		tcs, _ := msg["tool_calls"].([]interface{})
		for _, t := range tcs {
			tc, ok := t.(map[string]interface{})
			if !ok {
				continue
			}
			fn, _ := tc["function"].(map[string]interface{})
			if fn == nil {
				continue
			}
			id, _ := tc["id"].(string)
			name, _ := fn["name"].(string)
			if id != "" && name != "" {
				m[id] = name
			}
		}
	}
	return m
}

// buildToolResponseCache maps tool_call_id → content from tool messages.
func buildToolResponseCache(body map[string]interface{}) map[string]string {
	cache := map[string]string{}
	msgs, _ := body["messages"].([]interface{})
	for _, raw := range msgs {
		msg, ok := raw.(map[string]interface{})
		if !ok || msg["role"] != "tool" {
			continue
		}
		id, _ := msg["tool_call_id"].(string)
		if id != "" {
			cache[id] = strVal(msg["content"])
		}
	}
	return cache
}

// convertMessagesToGemini converts OpenAI messages to Gemini contents array.
func convertMessagesToGemini(body map[string]interface{}, tcID2Name, toolResponses map[string]string) ([]interface{}, interface{}) {
	msgs, _ := body["messages"].([]interface{})
	var contents []interface{}
	var systemInstruction interface{}

	for _, raw := range msgs {
		msg, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := msg["role"].(string)
		content := msg["content"]

		switch role {
		case "system":
			text := extractText(content)
			if text != "" {
				systemInstruction = map[string]interface{}{
					"role":  "user",
					"parts": []interface{}{map[string]interface{}{"text": text}},
				}
			}

		case "user":
			parts := openAIContentToGeminiParts(content)
			if len(parts) > 0 {
				contents = append(contents, map[string]interface{}{"role": "user", "parts": parts})
			}

		case "assistant":
			parts := assistantToGeminiParts(msg, tcID2Name, toolResponses, &contents)
			if len(parts) > 0 {
				contents = append(contents, map[string]interface{}{"role": "model", "parts": parts})
			}
		}
	}

	return contents, systemInstruction
}

// assistantToGeminiParts converts an assistant message to Gemini model parts, and may
// append a corresponding tool response turn to contents.
func assistantToGeminiParts(msg map[string]interface{}, tcID2Name, toolResponses map[string]string, contents *[]interface{}) []interface{} {
	var parts []interface{}

	if text := extractText(msg["content"]); text != "" {
		parts = append(parts, map[string]interface{}{"text": text})
	}

	tcs, _ := msg["tool_calls"].([]interface{})
	if len(tcs) == 0 {
		return parts
	}

	var toolCallIDs []string
	for _, t := range tcs {
		tc, ok := t.(map[string]interface{})
		if !ok {
			continue
		}
		fn, _ := tc["function"].(map[string]interface{})
		if fn == nil {
			continue
		}
		id, _ := tc["id"].(string)
		name, _ := fn["name"].(string)
		args := parseJSONArg(strVal(fn["arguments"]))
		parts = append(parts, map[string]interface{}{
			"functionCall": map[string]interface{}{
				"id":   id,
				"name": name,
				"args": args,
			},
		})
		toolCallIDs = append(toolCallIDs, id)
	}

	// Append tool response turn if responses exist
	var toolParts []interface{}
	for _, fid := range toolCallIDs {
		resp, exists := toolResponses[fid]
		if !exists {
			continue
		}
		name := tcID2Name[fid]
		if name == "" {
			name = fid
		}
		var parsed interface{}
		if err := json.Unmarshal([]byte(resp), &parsed); err != nil {
			parsed = map[string]interface{}{"result": resp}
		} else if parsed == nil {
			parsed = map[string]interface{}{"result": resp}
		}
		toolParts = append(toolParts, map[string]interface{}{
			"functionResponse": map[string]interface{}{
				"id":   fid,
				"name": name,
				"response": map[string]interface{}{"result": parsed},
			},
		})
	}
	if len(toolParts) > 0 {
		*contents = append(*contents, map[string]interface{}{"role": "user", "parts": toolParts})
	}

	return parts
}

// openAIContentToGeminiParts converts OpenAI user content to Gemini parts array.
func openAIContentToGeminiParts(content interface{}) []interface{} {
	var parts []interface{}
	switch v := content.(type) {
	case string:
		if v != "" {
			parts = append(parts, map[string]interface{}{"text": v})
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
					parts = append(parts, map[string]interface{}{"text": text})
				}
			case "image_url":
				if imgPart := imageURLToGeminiPart(part); imgPart != nil {
					parts = append(parts, imgPart)
				}
			}
		}
	}
	return parts
}

// imageURLToGeminiPart converts an OpenAI image_url content part to a Gemini inlineData part.
func imageURLToGeminiPart(part map[string]interface{}) map[string]interface{} {
	imgURL, _ := part["image_url"].(map[string]interface{})
	if imgURL == nil {
		return nil
	}
	url, _ := imgURL["url"].(string)
	if len(url) < 5 || url[:5] != "data:" {
		return nil
	}
	semi := indexOf(url, ';')
	comma := indexOf(url, ',')
	if semi <= 0 || comma <= semi {
		return nil
	}
	mimeType := url[5:semi]
	data := url[comma+1:]
	return map[string]interface{}{
		"inlineData": map[string]interface{}{
			"mimeType": mimeType,
			"data":     data,
		},
	}
}

// toolsToFunctionDeclarations converts OpenAI tools to Gemini functionDeclarations.
func toolsToFunctionDeclarations(tools []interface{}) []interface{} {
	var decls []interface{}
	for _, t := range tools {
		tool, ok := t.(map[string]interface{})
		if !ok {
			continue
		}
		var fn map[string]interface{}
		if toolType, _ := tool["type"].(string); toolType == "function" {
			fn, _ = tool["function"].(map[string]interface{})
		} else if tool["name"] != nil && tool["input_schema"] != nil {
			fn = tool
		}
		if fn == nil {
			continue
		}
		params := fn["parameters"]
		if params == nil {
			params = fn["input_schema"]
		}
		if params == nil {
			params = map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
		}
		decls = append(decls, map[string]interface{}{
			"name":        strVal(fn["name"]),
			"description": strVal(fn["description"]),
			"parameters":  params,
		})
	}
	return decls
}
