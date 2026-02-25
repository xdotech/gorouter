// Package types defines shared types for the translator subsystem.
package types

// SSEEvent represents a Server-Sent Events message with optional event type.
type SSEEvent struct {
	Event string
	Data  string
}

// Usage holds token usage counts from a completed LLM response.
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// StreamTranslator translates upstream SSE chunks into OpenAI-format SSE events.
type StreamTranslator interface {
	// TranslateEvent translates a single upstream SSE data line to zero or more OpenAI events.
	TranslateEvent(line string) ([]SSEEvent, error)
	// Flush emits any remaining buffered events at end-of-stream.
	Flush() ([]SSEEvent, error)
	// GetUsage returns accumulated token usage after stream completes.
	GetUsage() *Usage
}
