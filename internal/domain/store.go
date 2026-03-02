package domain

// ConnectionStore manages provider connections.
type ConnectionStore interface {
	List(filter ConnectionFilter) ([]ProviderConnection, error)
	Get(id string) (*ProviderConnection, error)
	Create(conn ProviderConnection) error
	Update(id string, updates map[string]interface{}) error
	Delete(id string) error
}

// NodeStore manages custom provider nodes.
type NodeStore interface {
	List() ([]ProviderNode, error)
	Create(node ProviderNode) error
	Update(id string, node ProviderNode) error
	Delete(id string) error
}

// ComboStore manages model combo chains.
type ComboStore interface {
	List() ([]Combo, error)
	Create(combo Combo) error
	Update(id string, combo Combo) error
	Delete(id string) error
}

// APIKeyStore manages local API keys.
type APIKeyStore interface {
	List() ([]APIKey, error)
	Create(key APIKey) error
	Delete(id string) error
	ValidateKey(key string) bool
}

// SettingsStore manages runtime settings.
type SettingsStore interface {
	Get() (*Settings, error)
	Update(updates map[string]interface{}) error
}

// AliasStore manages model name aliases.
type AliasStore interface {
	List() (map[string]string, error)
	Set(alias, target string) error
	Delete(alias string) error
}

// PricingStore manages token pricing configuration.
type PricingStore interface {
	Get() (map[string]float64, error)
	Update(pricing map[string]float64) error
}

// Stores aggregates all storage interfaces.
// Consumers depend on this struct rather than concrete implementations.
type Stores struct {
	Connections ConnectionStore
	Nodes       NodeStore
	Combos      ComboStore
	APIKeys     APIKeyStore
	Settings    SettingsStore
	Aliases     AliasStore
	Pricing     PricingStore
}
