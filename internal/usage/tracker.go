package usage

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Tracker wires DB + Logger + Broadcaster into one call.
type Tracker struct {
	db          *DB
	logger      *Logger
	broadcaster *Broadcaster
	pricing     map[string]float64
}

// NewTracker creates a Tracker. pricing may be nil (uses defaults).
func NewTracker(db *DB, logger *Logger, pricing map[string]float64) *Tracker {
	return &Tracker{
		db:          db,
		logger:      logger,
		broadcaster: Global,
		pricing:     pricing,
	}
}

// Record saves a usage entry and broadcasts it.
func (t *Tracker) Record(
	provider, model, connectionID, endpoint string,
	promptTokens, completionTokens, statusCode int,
	durationMs int64,
	isStreaming bool,
) {
	total := promptTokens + completionTokens
	cost := EstimateCost(provider, model, promptTokens, completionTokens, t.pricing)

	e := Entry{
		ID:               uuid.New().String(),
		Provider:         provider,
		Model:            model,
		ConnectionID:     connectionID,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      total,
		EstimatedCost:    cost,
		DurationMs:       durationMs,
		Timestamp:        time.Now().UTC().Format(time.RFC3339),
		RequestID:        fmt.Sprintf("req_%d", time.Now().UnixNano()),
		IsStreaming:      isStreaming,
		StatusCode:       statusCode,
		Endpoint:         endpoint,
	}

	_ = t.db.Append(e)
	t.logger.Log(e)
	t.broadcaster.Publish(e)
}

// UpdatePricing updates the pricing table (called when user changes pricing in dashboard).
func (t *Tracker) UpdatePricing(pricing map[string]float64) {
	t.pricing = pricing
}
