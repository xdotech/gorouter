package usage

// DefaultPricing holds input/output cost per 1M tokens for known providers.
// Cost is in USD.
var DefaultPricing = map[string][2]float64{
	// provider/model → [inputPer1M, outputPer1M]
	"glm/glm-4.7":              {0.6, 0.6},
	"minimax/MiniMax-M2.1":     {0.2, 0.2},
	"kimi/kimi-latest":         {9.0, 9.0},
	"openai/gpt-4o":            {5.0, 15.0},
	"openai/gpt-4o-mini":       {0.15, 0.6},
	"anthropic/claude-opus-4-6":{15.0, 75.0},
	// Free providers = 0 cost (default)
}

// EstimateCost calculates USD cost for a request.
// customPricing overrides defaults (keyed by "provider/model" or just "provider").
func EstimateCost(provider, model string, promptTokens, completionTokens int, customPricing map[string]float64) float64 {
	key := provider + "/" + model

	// Check custom pricing first
	if customPricing != nil {
		if price, ok := customPricing[key]; ok {
			total := float64(promptTokens+completionTokens) / 1_000_000 * price
			return roundCost(total)
		}
		if price, ok := customPricing[provider]; ok {
			total := float64(promptTokens+completionTokens) / 1_000_000 * price
			return roundCost(total)
		}
	}

	// Check defaults
	if prices, ok := DefaultPricing[key]; ok {
		input := float64(promptTokens) / 1_000_000 * prices[0]
		output := float64(completionTokens) / 1_000_000 * prices[1]
		return roundCost(input + output)
	}

	// Free providers
	switch provider {
	case "if", "iflow", "qw", "qwen", "kr", "kiro", "gc", "gemini-cli":
		return 0
	}

	return 0
}

func roundCost(v float64) float64 {
	// Round to 6 decimal places
	factor := 1_000_000.0
	return float64(int64(v*factor+0.5)) / factor
}
