// Package jsonfile implements domain.Stores backed by a single JSON file.
// It preserves the existing db.json format for backwards compatibility.
package jsonfile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/xdotech/gorouter/internal/domain"
)

// dbData is the root JSON structure for db.json.
type dbData struct {
	ProviderConnections []domain.ProviderConnection `json:"providerConnections"`
	ProviderNodes       []domain.ProviderNode       `json:"providerNodes"`
	ModelAliases        map[string]string           `json:"modelAliases"`
	MitmAlias           map[string]string           `json:"mitmAlias"`
	Combos              []domain.Combo              `json:"combos"`
	APIKeys             []domain.APIKey             `json:"apiKeys"`
	Settings            domain.Settings             `json:"settings"`
	Pricing             map[string]float64          `json:"pricing"`
}

// Store is the main database backed by a JSON file.
type Store struct {
	mu   sync.RWMutex
	file string
	data dbData
}

// New opens (or creates) the JSON store at the given data directory.
// Returns a domain.Stores aggregate that satisfies all storage interfaces.
func New(dataDir string) (*Store, *domain.Stores, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, nil, fmt.Errorf("create data dir: %w", err)
	}

	file := filepath.Join(dataDir, "db.json")
	s := &Store{file: file}

	if err := s.load(); err != nil {
		return nil, nil, err
	}

	stores := &domain.Stores{
		Connections: s,
		Nodes:       s.nodeStore(),
		Combos:      s.comboStore(),
		APIKeys:     s.apiKeyStore(),
		Settings:    s.settingsStore(),
		Aliases:     s.aliasStore(),
		Pricing:     s.pricingStore(),
	}

	return s, stores, nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.file)
	if os.IsNotExist(err) {
		s.data = defaultDBData()
		return s.save()
	}
	if err != nil {
		return fmt.Errorf("read db.json: %w", err)
	}
	if err := json.Unmarshal(data, &s.data); err != nil {
		s.data = defaultDBData()
		return s.save()
	}
	s.repair()
	return nil
}

func (s *Store) repair() {
	if s.data.ModelAliases == nil {
		s.data.ModelAliases = make(map[string]string)
	}
	if s.data.MitmAlias == nil {
		s.data.MitmAlias = make(map[string]string)
	}
	if s.data.Pricing == nil {
		s.data.Pricing = make(map[string]float64)
	}
	if s.data.Settings.StickyRoundRobinLimit == 0 {
		s.data.Settings.StickyRoundRobinLimit = 3
	}
	if s.data.Settings.FallbackStrategy == "" {
		s.data.Settings.FallbackStrategy = "fill-first"
	}
	if s.data.Settings.ObservabilityMaxRecords == 0 {
		s.data.Settings.ObservabilityMaxRecords = 1000
	}
}

func (s *Store) save() error {
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal db: %w", err)
	}
	tmp := s.file + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("write db.json: %w", err)
	}
	return os.Rename(tmp, s.file)
}

func defaultDBData() dbData {
	return dbData{
		ProviderConnections: []domain.ProviderConnection{},
		ProviderNodes:       []domain.ProviderNode{},
		ModelAliases:        make(map[string]string),
		MitmAlias:           make(map[string]string),
		Combos:              []domain.Combo{},
		APIKeys:             []domain.APIKey{},
		Settings:            domain.DefaultSettings(),
		Pricing:             make(map[string]float64),
	}
}

// ─── ConnectionStore ────────────────────────────────────────────────────────

func (s *Store) List(filter domain.ConnectionFilter) ([]domain.ProviderConnection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []domain.ProviderConnection
	for _, c := range s.data.ProviderConnections {
		if filter.Provider != "" && c.Provider != filter.Provider {
			continue
		}
		if filter.IsActive != nil && c.IsActive != *filter.IsActive {
			continue
		}
		out = append(out, c)
	}
	return out, nil
}

func (s *Store) Get(id string) (*domain.ProviderConnection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range s.data.ProviderConnections {
		if s.data.ProviderConnections[i].ID == id {
			c := s.data.ProviderConnections[i]
			return &c, nil
		}
	}
	return nil, fmt.Errorf("connection %s not found", id)
}

func (s *Store) Create(conn domain.ProviderConnection) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.ProviderConnections = append(s.data.ProviderConnections, conn)
	return s.save()
}

func (s *Store) Update(id string, updates map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.data.ProviderConnections {
		if s.data.ProviderConnections[i].ID != id {
			continue
		}
		c := &s.data.ProviderConnections[i]
		applyConnectionUpdates(c, updates)
		return s.save()
	}
	return fmt.Errorf("connection %s not found", id)
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	conns := s.data.ProviderConnections[:0]
	for _, c := range s.data.ProviderConnections {
		if c.ID != id {
			conns = append(conns, c)
		}
	}
	s.data.ProviderConnections = conns
	return s.save()
}

func applyConnectionUpdates(c *domain.ProviderConnection, updates map[string]interface{}) {
	for k, v := range updates {
		switch k {
		case "isActive":
			if b, ok := v.(bool); ok {
				c.IsActive = b
			}
		case "testStatus":
			if s, ok := v.(string); ok {
				c.TestStatus = s
			}
		case "lastError":
			if s, ok := v.(string); ok {
				c.LastError = s
			} else {
				c.LastError = ""
			}
		case "lastErrorAt":
			if s, ok := v.(string); ok {
				c.LastErrorAt = s
			} else {
				c.LastErrorAt = ""
			}
		case "errorCode":
			switch n := v.(type) {
			case int:
				c.ErrorCode = n
			case float64:
				c.ErrorCode = int(n)
			}
		case "rateLimitedUntil":
			if s, ok := v.(string); ok {
				c.RateLimitedUntil = s
			} else {
				c.RateLimitedUntil = ""
			}
		case "backoffLevel":
			switch n := v.(type) {
			case int:
				c.BackoffLevel = n
			case float64:
				c.BackoffLevel = int(n)
			}
		case "accessToken":
			if s, ok := v.(string); ok {
				c.AccessToken = s
			}
		case "refreshToken":
			if s, ok := v.(string); ok {
				c.RefreshToken = s
			}
		case "expiresAt":
			if s, ok := v.(string); ok {
				c.ExpiresAt = s
			}
		case "projectId":
			if s, ok := v.(string); ok {
				c.ProjectID = s
			}
		case "lastUsedAt":
			if s, ok := v.(string); ok {
				c.LastUsedAt = s
			}
		case "consecutiveUseCount":
			switch n := v.(type) {
			case int:
				c.ConsecutiveUseCount = n
			case float64:
				c.ConsecutiveUseCount = int(n)
			}
		case "providerSpecificData":
			if m, ok := v.(map[string]interface{}); ok {
				c.ProviderSpecificData = m
			}
		case "priority":
			switch n := v.(type) {
			case int:
				c.Priority = n
			case float64:
				c.Priority = int(n)
			}
		case "name":
			if s, ok := v.(string); ok {
				c.Name = s
			}
		case "apiKey":
			if s, ok := v.(string); ok {
				c.APIKey = s
			}
		}
	}
}

// ─── NodeStore ───────────────────────────────────────────────────────────────

// Compile-time check: Store must implement domain.NodeStore.
// Method names would collide with ConnectionStore (List, Create, etc.),
// so NodeStore uses distinct method names via wrapper.

type nodeStoreAdapter struct{ s *Store }

func (s *Store) nodeStore() domain.NodeStore { return &nodeStoreAdapter{s: s} }

func (a *nodeStoreAdapter) List() ([]domain.ProviderNode, error) {
	a.s.mu.RLock()
	defer a.s.mu.RUnlock()
	out := make([]domain.ProviderNode, len(a.s.data.ProviderNodes))
	copy(out, a.s.data.ProviderNodes)
	return out, nil
}

func (a *nodeStoreAdapter) Create(node domain.ProviderNode) error {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	a.s.data.ProviderNodes = append(a.s.data.ProviderNodes, node)
	return a.s.save()
}

func (a *nodeStoreAdapter) Update(id string, node domain.ProviderNode) error {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	for i := range a.s.data.ProviderNodes {
		if a.s.data.ProviderNodes[i].ID == id {
			a.s.data.ProviderNodes[i] = node
			return a.s.save()
		}
	}
	return fmt.Errorf("provider node %s not found", id)
}

func (a *nodeStoreAdapter) Delete(id string) error {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	nodes := a.s.data.ProviderNodes[:0]
	for _, n := range a.s.data.ProviderNodes {
		if n.ID != id {
			nodes = append(nodes, n)
		}
	}
	a.s.data.ProviderNodes = nodes
	return a.s.save()
}

// ─── ComboStore ──────────────────────────────────────────────────────────────

type comboStoreAdapter struct{ s *Store }

func (s *Store) comboStore() domain.ComboStore { return &comboStoreAdapter{s: s} }

func (a *comboStoreAdapter) List() ([]domain.Combo, error) {
	a.s.mu.RLock()
	defer a.s.mu.RUnlock()
	out := make([]domain.Combo, len(a.s.data.Combos))
	copy(out, a.s.data.Combos)
	return out, nil
}

func (a *comboStoreAdapter) Create(combo domain.Combo) error {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	a.s.data.Combos = append(a.s.data.Combos, combo)
	return a.s.save()
}

func (a *comboStoreAdapter) Update(id string, combo domain.Combo) error {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	for i := range a.s.data.Combos {
		if a.s.data.Combos[i].ID == id {
			a.s.data.Combos[i] = combo
			return a.s.save()
		}
	}
	return fmt.Errorf("combo %s not found", id)
}

func (a *comboStoreAdapter) Delete(id string) error {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	combos := a.s.data.Combos[:0]
	for _, c := range a.s.data.Combos {
		if c.ID != id {
			combos = append(combos, c)
		}
	}
	a.s.data.Combos = combos
	return a.s.save()
}

// GetComboByName looks up a combo by name (used by the routing layer).
func (s *Store) GetComboByName(name string) (*domain.Combo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.data.Combos {
		if c.Name == name {
			combo := c
			return &combo, nil
		}
	}
	return nil, nil
}

// ─── APIKeyStore ─────────────────────────────────────────────────────────────

type apiKeyStoreAdapter struct{ s *Store }

func (s *Store) apiKeyStore() domain.APIKeyStore { return &apiKeyStoreAdapter{s: s} }

func (a *apiKeyStoreAdapter) List() ([]domain.APIKey, error) {
	a.s.mu.RLock()
	defer a.s.mu.RUnlock()
	out := make([]domain.APIKey, len(a.s.data.APIKeys))
	copy(out, a.s.data.APIKeys)
	return out, nil
}

func (a *apiKeyStoreAdapter) Create(key domain.APIKey) error {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	a.s.data.APIKeys = append(a.s.data.APIKeys, key)
	return a.s.save()
}

func (a *apiKeyStoreAdapter) Delete(id string) error {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	keys := a.s.data.APIKeys[:0]
	for _, k := range a.s.data.APIKeys {
		if k.ID != id {
			keys = append(keys, k)
		}
	}
	a.s.data.APIKeys = keys
	return a.s.save()
}

func (a *apiKeyStoreAdapter) ValidateKey(key string) bool {
	a.s.mu.RLock()
	defer a.s.mu.RUnlock()
	for _, k := range a.s.data.APIKeys {
		if k.Key == key && k.IsActive {
			return true
		}
	}
	return false
}

// ─── SettingsStore ───────────────────────────────────────────────────────────

type settingsStoreAdapter struct{ s *Store }

func (s *Store) settingsStore() domain.SettingsStore { return &settingsStoreAdapter{s: s} }

func (a *settingsStoreAdapter) Get() (*domain.Settings, error) {
	a.s.mu.RLock()
	defer a.s.mu.RUnlock()
	st := a.s.data.Settings
	return &st, nil
}

func (a *settingsStoreAdapter) Update(updates map[string]interface{}) error {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	applySettingsUpdates(&a.s.data.Settings, updates)
	return a.s.save()
}

func applySettingsUpdates(st *domain.Settings, updates map[string]interface{}) {
	for k, v := range updates {
		switch k {
		case "cloudEnabled":
			if b, ok := v.(bool); ok {
				st.CloudEnabled = b
			}
		case "requireLogin":
			if b, ok := v.(bool); ok {
				st.RequireLogin = b
			}
		case "requireApiKey":
			if b, ok := v.(bool); ok {
				st.RequireAPIKey = b
			}
		case "passwordHash":
			if s, ok := v.(string); ok {
				st.PasswordHash = s
			}
		case "fallbackStrategy":
			if s, ok := v.(string); ok {
				st.FallbackStrategy = s
			}
		case "stickyRoundRobinLimit":
			switch n := v.(type) {
			case int:
				st.StickyRoundRobinLimit = n
			case float64:
				st.StickyRoundRobinLimit = int(n)
			}
		case "observabilityEnabled":
			if b, ok := v.(bool); ok {
				st.ObservabilityEnabled = b
			}
		case "observabilityMaxRecords":
			switch n := v.(type) {
			case int:
				st.ObservabilityMaxRecords = n
			case float64:
				st.ObservabilityMaxRecords = int(n)
			}
		}
	}
}

// ─── AliasStore ──────────────────────────────────────────────────────────────

type aliasStoreAdapter struct{ s *Store }

func (s *Store) aliasStore() domain.AliasStore { return &aliasStoreAdapter{s: s} }

func (a *aliasStoreAdapter) List() (map[string]string, error) {
	a.s.mu.RLock()
	defer a.s.mu.RUnlock()
	out := make(map[string]string, len(a.s.data.ModelAliases))
	for k, v := range a.s.data.ModelAliases {
		out[k] = v
	}
	return out, nil
}

func (a *aliasStoreAdapter) Set(alias, target string) error {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	a.s.data.ModelAliases[alias] = target
	return a.s.save()
}

func (a *aliasStoreAdapter) Delete(alias string) error {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	delete(a.s.data.ModelAliases, alias)
	return a.s.save()
}

// ─── PricingStore ────────────────────────────────────────────────────────────

type pricingStoreAdapter struct{ s *Store }

func (s *Store) pricingStore() domain.PricingStore { return &pricingStoreAdapter{s: s} }

func (a *pricingStoreAdapter) Get() (map[string]float64, error) {
	a.s.mu.RLock()
	defer a.s.mu.RUnlock()
	out := make(map[string]float64, len(a.s.data.Pricing))
	for k, v := range a.s.data.Pricing {
		out[k] = v
	}
	return out, nil
}

func (a *pricingStoreAdapter) Update(pricing map[string]float64) error {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	a.s.data.Pricing = pricing
	return a.s.save()
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// Now returns the current UTC time as an RFC3339 string.
func Now() string { return time.Now().UTC().Format(time.RFC3339) }
