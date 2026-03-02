package db

// ProviderConnection holds auth credentials + state for a single provider account.
type ProviderConnection struct {
	ID                   string                 `json:"id"`
	Provider             string                 `json:"provider"`
	AuthType             string                 `json:"authType"` // "oauth" | "apikey"
	Name                 string                 `json:"name"`
	Priority             int                    `json:"priority"`
	IsActive             bool                   `json:"isActive"`
	APIKey               string                 `json:"apiKey,omitempty"`
	AccessToken          string                 `json:"accessToken,omitempty"`
	RefreshToken         string                 `json:"refreshToken,omitempty"`
	ExpiresAt            string                 `json:"expiresAt,omitempty"`
	ProjectID            string                 `json:"projectId,omitempty"`
	TestStatus           string                 `json:"testStatus"` // "active" | "unavailable" | "unknown"
	LastError            string                 `json:"lastError,omitempty"`
	ErrorCode            int                    `json:"errorCode,omitempty"`
	LastErrorAt          string                 `json:"lastErrorAt,omitempty"`
	RateLimitedUntil     string                 `json:"rateLimitedUntil,omitempty"`
	BackoffLevel         int                    `json:"backoffLevel"`
	LastUsedAt           string                 `json:"lastUsedAt,omitempty"`
	ConsecutiveUseCount  int                    `json:"consecutiveUseCount"`
	ProviderSpecificData map[string]interface{} `json:"providerSpecificData,omitempty"`
}

// ProviderNode is a custom OpenAI/Anthropic-compatible endpoint configured by the user.
type ProviderNode struct {
	ID      string `json:"id"`
	Type    string `json:"type"` // "openai-compatible" | "anthropic-compatible"
	Name    string `json:"name"`
	Prefix  string `json:"prefix"`
	APIType string `json:"apiType"`
	BaseURL string `json:"baseUrl"`
}

// Combo is a named sequence of models with automatic fallback.
type Combo struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Models []string `json:"models"`
}

// APIKey is a local API key for authenticating clients against the gateway.
type APIKey struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Key       string `json:"key"`
	MachineID string `json:"machineId"`
	IsActive  bool   `json:"isActive"`
	CreatedAt string `json:"createdAt"`
}

// Settings holds all runtime configuration.
type Settings struct {
	CloudEnabled                 bool   `json:"cloudEnabled"`
	TunnelEnabled                bool   `json:"tunnelEnabled"`
	TunnelURL                    string `json:"tunnelUrl"`
	FallbackStrategy             string `json:"fallbackStrategy"` // "fill-first" | "round-robin"
	StickyRoundRobinLimit        int    `json:"stickyRoundRobinLimit"`
	RequireLogin                 bool   `json:"requireLogin"`
	RequireAPIKey                bool   `json:"requireApiKey"`
	PasswordHash                 string `json:"passwordHash,omitempty"`
	ObservabilityEnabled         bool   `json:"observabilityEnabled"`
	ObservabilityMaxRecords      int    `json:"observabilityMaxRecords"`
	ObservabilityBatchSize       int    `json:"observabilityBatchSize"`
	ObservabilityFlushIntervalMs int    `json:"observabilityFlushIntervalMs"`
	ObservabilityMaxJSONSize     int    `json:"observabilityMaxJsonSize"`
}

// DBData is the root JSON structure for db.json.
type DBData struct {
	ProviderConnections []ProviderConnection `json:"providerConnections"`
	ProviderNodes       []ProviderNode       `json:"providerNodes"`
	ModelAliases        map[string]string    `json:"modelAliases"`
	MitmAlias           map[string]string    `json:"mitmAlias"`
	Combos              []Combo              `json:"combos"`
	APIKeys             []APIKey             `json:"apiKeys"`
	Settings            Settings             `json:"settings"`
	Pricing             map[string]float64   `json:"pricing"`
}

// ConnectionFilter is used to query provider connections.
type ConnectionFilter struct {
	Provider string
	IsActive *bool
}

func defaultSettings() Settings {
	return Settings{
		CloudEnabled:                 false,
		TunnelEnabled:                false,
		FallbackStrategy:             "fill-first",
		StickyRoundRobinLimit:        3,
		RequireLogin:                 false,
		RequireAPIKey:                false,
		ObservabilityEnabled:         true,
		ObservabilityMaxRecords:      1000,
		ObservabilityBatchSize:       20,
		ObservabilityFlushIntervalMs: 5000,
		ObservabilityMaxJSONSize:     1024,
	}
}
