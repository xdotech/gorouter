package router

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/xuando/gorouter/internal/translator/types"
)

// StreamResponse pipes upstream SSE body to the client, applying translation.
// Returns (promptTokens, completionTokens).
func StreamResponse(w http.ResponseWriter, upstreamBody io.ReadCloser, t types.StreamTranslator) (int, int) {
	defer upstreamBody.Close()

	flusher, canFlush := w.(http.Flusher)
	scanner := bufio.NewScanner(upstreamBody)

	for scanner.Scan() {
		line := scanner.Text()

		if t == nil {
			// Passthrough: write line as-is.
			writeSSELine(w, line)
			if canFlush {
				flusher.Flush()
			}
			continue
		}

		// Strip "data: " prefix before passing to translator.
		data := line
		if strings.HasPrefix(line, "data: ") {
			data = strings.TrimPrefix(line, "data: ")
		} else if line == "" || strings.HasPrefix(line, ":") {
			// Keep-alive or comment — write as-is.
			writeSSELine(w, line)
			if canFlush {
				flusher.Flush()
			}
			continue
		}

		events, err := t.TranslateEvent(data)
		if err != nil {
			continue
		}
		for _, ev := range events {
			writeSSEEvent(w, ev)
		}
		if canFlush {
			flusher.Flush()
		}
	}

	// Flush remaining buffered events.
	if t != nil {
		events, _ := t.Flush()
		for _, ev := range events {
			writeSSEEvent(w, ev)
		}
		if canFlush {
			flusher.Flush()
		}

		if usage := t.GetUsage(); usage != nil {
			return usage.PromptTokens, usage.CompletionTokens
		}
	}

	return 0, 0
}

func writeSSELine(w http.ResponseWriter, line string) {
	_, _ = w.Write([]byte(line + "\n"))
}

func writeSSEEvent(w http.ResponseWriter, ev types.SSEEvent) {
	if ev.Event != "" {
		_, _ = w.Write([]byte("event: " + ev.Event + "\n"))
	}
	_, _ = w.Write([]byte("data: " + ev.Data + "\n\n"))
}

// WriteJSONResponse translates a non-streaming upstream response to OpenAI format and writes it.
// Returns (promptTokens, completionTokens).
func WriteJSONResponse(w http.ResponseWriter, upstreamBody []byte, statusCode int, targetFormat string) (int, int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = w.Write(upstreamBody)

	// Extract token counts from the body if it's already OpenAI format.
	var parsed map[string]interface{}
	if err := json.Unmarshal(upstreamBody, &parsed); err != nil {
		return 0, 0
	}

	usage, ok := parsed["usage"].(map[string]interface{})
	if !ok {
		return 0, 0
	}

	prompt := int(toFloat64(usage["prompt_tokens"]))
	completion := int(toFloat64(usage["completion_tokens"]))
	return prompt, completion
}

// WriteError writes a JSON error response.
func WriteError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	body, _ := json.Marshal(map[string]interface{}{
		"error": map[string]interface{}{
			"message": message,
			"type":    "api_error",
			"code":    statusCode,
		},
	})
	_, _ = w.Write(body)
}

func toFloat64(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	}
	return 0
}
