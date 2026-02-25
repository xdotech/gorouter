package usage

// Entry records a single completed AI request.
type Entry struct {
	ID               string  `json:"id"`
	Provider         string  `json:"provider"`
	Model            string  `json:"model"`
	ConnectionID     string  `json:"connectionId"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	EstimatedCost    float64 `json:"estimated_cost"`
	DurationMs       int64   `json:"duration_ms"`
	Timestamp        string  `json:"timestamp"`
	RequestID        string  `json:"requestId"`
	IsStreaming      bool    `json:"isStreaming"`
	StatusCode       int     `json:"statusCode"`
	Endpoint         string  `json:"endpoint"`
}

// ProviderStats is aggregated usage per provider.
type ProviderStats struct {
	Provider         string  `json:"provider"`
	TotalRequests    int     `json:"totalRequests"`
	PromptTokens     int     `json:"promptTokens"`
	CompletionTokens int     `json:"completionTokens"`
	TotalTokens      int     `json:"totalTokens"`
	EstimatedCost    float64 `json:"estimatedCost"`
}

// RequestDetail holds the full request/response payload for a single request.
type RequestDetail struct {
	ID           string `json:"id"`
	Timestamp    string `json:"timestamp"`
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	StatusCode   int    `json:"statusCode"`
	DurationMs   int64  `json:"duration_ms"`
	Endpoint     string `json:"endpoint"`
	RequestBody  []byte `json:"requestBody,omitempty"`
	ResponseBody []byte `json:"responseBody,omitempty"`
}

// usageData is the root JSON structure for usage.json.
type usageData struct {
	Entries []Entry `json:"entries"`
}
