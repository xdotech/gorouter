package db

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Store is the main database backed by a JSON file.
type Store struct {
	mu   sync.RWMutex
	file string
	data DBData
}

// New opens (or creates) the JSON store at the given data directory.
func New(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	file := filepath.Join(dataDir, "db.json")
	s := &Store{file: file}

	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
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
		// Corrupt JSON — reset to defaults
		s.data = defaultDBData()
		return s.save()
	}

	s.repair()
	return nil
}

// repair fills in missing fields from older DB versions.
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
	// Write atomically via temp file
	tmp := s.file + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("write db.json: %w", err)
	}
	return os.Rename(tmp, s.file)
}

func defaultDBData() DBData {
	return DBData{
		ProviderConnections: []ProviderConnection{},
		ProviderNodes:       []ProviderNode{},
		ModelAliases:        make(map[string]string),
		MitmAlias:           make(map[string]string),
		Combos:              []Combo{},
		APIKeys:             []APIKey{},
		Settings:            defaultSettings(),
		Pricing:             make(map[string]float64),
	}
}

// ─── Provider Connections ───────────────────────────────────────────────────

func (s *Store) GetProviderConnections(f ConnectionFilter) ([]ProviderConnection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []ProviderConnection
	for _, c := range s.data.ProviderConnections {
		if f.Provider != "" && c.Provider != f.Provider {
			continue
		}
		if f.IsActive != nil && c.IsActive != *f.IsActive {
			continue
		}
		out = append(out, c)
	}
	return out, nil
}

func (s *Store) GetProviderConnection(id string) (*ProviderConnection, error) {
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

func (s *Store) CreateProviderConnection(conn ProviderConnection) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.ProviderConnections = append(s.data.ProviderConnections, conn)
	return s.save()
}

func (s *Store) UpdateProviderConnection(id string, updates map[string]interface{}) error {
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

func (s *Store) DeleteProviderConnection(id string) error {
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

func applyConnectionUpdates(c *ProviderConnection, updates map[string]interface{}) {
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

// ─── Provider Nodes ─────────────────────────────────────────────────────────

func (s *Store) GetProviderNodes() ([]ProviderNode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ProviderNode, len(s.data.ProviderNodes))
	copy(out, s.data.ProviderNodes)
	return out, nil
}

func (s *Store) CreateProviderNode(node ProviderNode) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.ProviderNodes = append(s.data.ProviderNodes, node)
	return s.save()
}

func (s *Store) UpdateProviderNode(id string, node ProviderNode) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.ProviderNodes {
		if s.data.ProviderNodes[i].ID == id {
			s.data.ProviderNodes[i] = node
			return s.save()
		}
	}
	return fmt.Errorf("provider node %s not found", id)
}

func (s *Store) DeleteProviderNode(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	nodes := s.data.ProviderNodes[:0]
	for _, n := range s.data.ProviderNodes {
		if n.ID != id {
			nodes = append(nodes, n)
		}
	}
	s.data.ProviderNodes = nodes
	return s.save()
}

// ─── Combos ─────────────────────────────────────────────────────────────────

func (s *Store) GetCombos() ([]Combo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Combo, len(s.data.Combos))
	copy(out, s.data.Combos)
	return out, nil
}

func (s *Store) GetCombo(id string) (*Combo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.data.Combos {
		if c.ID == id {
			combo := c
			return &combo, nil
		}
	}
	return nil, fmt.Errorf("combo %s not found", id)
}

func (s *Store) GetComboByName(name string) (*Combo, error) {
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

func (s *Store) CreateCombo(combo Combo) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Combos = append(s.data.Combos, combo)
	return s.save()
}

func (s *Store) UpdateCombo(id string, combo Combo) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Combos {
		if s.data.Combos[i].ID == id {
			s.data.Combos[i] = combo
			return s.save()
		}
	}
	return fmt.Errorf("combo %s not found", id)
}

func (s *Store) DeleteCombo(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	combos := s.data.Combos[:0]
	for _, c := range s.data.Combos {
		if c.ID != id {
			combos = append(combos, c)
		}
	}
	s.data.Combos = combos
	return s.save()
}

// ─── Model Aliases ───────────────────────────────────────────────────────────

func (s *Store) GetModelAliases() (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]string, len(s.data.ModelAliases))
	for k, v := range s.data.ModelAliases {
		out[k] = v
	}
	return out, nil
}

func (s *Store) SetModelAlias(alias, target string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.ModelAliases[alias] = target
	return s.save()
}

func (s *Store) DeleteModelAlias(alias string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data.ModelAliases, alias)
	return s.save()
}

// ─── API Keys ────────────────────────────────────────────────────────────────

func (s *Store) GetAPIKeys() ([]APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]APIKey, len(s.data.APIKeys))
	copy(out, s.data.APIKeys)
	return out, nil
}

func (s *Store) CreateAPIKey(key APIKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.APIKeys = append(s.data.APIKeys, key)
	return s.save()
}

func (s *Store) DeleteAPIKey(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	keys := s.data.APIKeys[:0]
	for _, k := range s.data.APIKeys {
		if k.ID != id {
			keys = append(keys, k)
		}
	}
	s.data.APIKeys = keys
	return s.save()
}

func (s *Store) ValidateAPIKey(key string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, k := range s.data.APIKeys {
		if k.Key == key && k.IsActive {
			return true, nil
		}
	}
	return false, nil
}

// ─── Settings ────────────────────────────────────────────────────────────────

func (s *Store) GetSettings() (Settings, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.Settings, nil
}

func (s *Store) UpdateSettings(updates map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	applySettingsUpdates(&s.data.Settings, updates)
	return s.save()
}

func applySettingsUpdates(st *Settings, updates map[string]interface{}) {
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

// ─── Pricing ─────────────────────────────────────────────────────────────────

func (s *Store) GetPricing() (map[string]float64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]float64, len(s.data.Pricing))
	for k, v := range s.data.Pricing {
		out[k] = v
	}
	return out, nil
}

func (s *Store) UpdatePricing(pricing map[string]float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Pricing = pricing
	return s.save()
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func Now() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func boolPtr(b bool) *bool { return &b }
