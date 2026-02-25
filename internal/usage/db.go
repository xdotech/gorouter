package usage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// DB manages usage.json with mutex-protected writes.
type DB struct {
	mu      sync.Mutex
	file    string
	entries []Entry
	maxSize int
}

// NewDB opens (or creates) the usage database.
func NewDB(dataDir string, maxSize int) (*DB, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	if maxSize <= 0 {
		maxSize = 1000
	}
	db := &DB{
		file:    filepath.Join(dataDir, "usage.json"),
		maxSize: maxSize,
	}
	if err := db.load(); err != nil {
		return nil, err
	}
	return db, nil
}

func (db *DB) load() error {
	data, err := os.ReadFile(db.file)
	if os.IsNotExist(err) {
		db.entries = []Entry{}
		return nil
	}
	if err != nil {
		return fmt.Errorf("read usage.json: %w", err)
	}

	var ud usageData
	if err := json.Unmarshal(data, &ud); err != nil {
		db.entries = []Entry{}
		return nil // reset on corrupt
	}
	db.entries = ud.Entries
	return nil
}

func (db *DB) save() error {
	ud := usageData{Entries: db.entries}
	data, err := json.Marshal(ud)
	if err != nil {
		return fmt.Errorf("marshal usage: %w", err)
	}
	tmp := db.file + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("write usage.json: %w", err)
	}
	return os.Rename(tmp, db.file)
}

// Append adds a usage entry, pruning oldest if over maxSize.
func (db *DB) Append(e Entry) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	db.entries = append(db.entries, e)
	if len(db.entries) > db.maxSize {
		db.entries = db.entries[len(db.entries)-db.maxSize:]
	}
	return db.save()
}

// GetAll returns a copy of all entries (newest-first).
func (db *DB) GetAll() []Entry {
	db.mu.Lock()
	defer db.mu.Unlock()

	out := make([]Entry, len(db.entries))
	copy(out, db.entries)
	// Reverse so newest is first
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// Aggregate returns per-provider statistics.
func (db *DB) Aggregate() []ProviderStats {
	db.mu.Lock()
	defer db.mu.Unlock()

	statsMap := make(map[string]*ProviderStats)
	for _, e := range db.entries {
		s, ok := statsMap[e.Provider]
		if !ok {
			s = &ProviderStats{Provider: e.Provider}
			statsMap[e.Provider] = s
		}
		s.TotalRequests++
		s.PromptTokens += e.PromptTokens
		s.CompletionTokens += e.CompletionTokens
		s.TotalTokens += e.TotalTokens
		s.EstimatedCost += e.EstimatedCost
	}

	out := make([]ProviderStats, 0, len(statsMap))
	for _, s := range statsMap {
		out = append(out, *s)
	}
	return out
}

// GetPage returns paginated entries (newest-first).
func (db *DB) GetPage(page, limit int) ([]Entry, int) {
	db.mu.Lock()
	defer db.mu.Unlock()

	total := len(db.entries)
	if limit <= 0 {
		limit = 20
	}

	// Reverse slice (newest first) without modifying original
	all := make([]Entry, total)
	for i, e := range db.entries {
		all[total-1-i] = e
	}

	start := (page - 1) * limit
	if start >= total {
		return []Entry{}, total
	}
	end := start + limit
	if end > total {
		end = total
	}
	return all[start:end], total
}
