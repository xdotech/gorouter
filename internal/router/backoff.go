package router

import "strings"

// fallbackStatuses are HTTP codes that should trigger account fallback.
var fallbackStatuses = map[int]bool{
	429: true,
	503: true,
	502: true,
	504: true,
	401: true,
	403: true,
}

// fallbackErrorPatterns are substrings in error text that trigger fallback.
var fallbackErrorPatterns = []string{
	"quota", "rate limit", "rate_limit", "overloaded", "capacity",
	"too many requests", "context length", "token limit", "exhausted",
}

// backoffLevels holds cooldown durations in milliseconds (exponential).
// Levels: 1min, 5min, 30min, 2h, 8h
var backoffLevels = []int64{60_000, 300_000, 1_800_000, 7_200_000, 28_800_000}

// FallbackDecision is returned by CheckFallbackError.
type FallbackDecision struct {
	ShouldFallback  bool
	CooldownMs      int64
	NewBackoffLevel int
}

// CheckFallbackError determines whether the error/status warrants a fallback
// and computes the next exponential backoff cooldown.
func CheckFallbackError(statusCode int, errorText string, backoffLevel int) FallbackDecision {
	if !shouldFallback(statusCode, errorText) {
		return FallbackDecision{ShouldFallback: false}
	}

	newLevel := backoffLevel + 1
	if newLevel >= len(backoffLevels) {
		newLevel = len(backoffLevels) - 1
	}
	cooldown := backoffLevels[newLevel]

	return FallbackDecision{
		ShouldFallback:  true,
		CooldownMs:      cooldown,
		NewBackoffLevel: newLevel,
	}
}

func shouldFallback(statusCode int, errorText string) bool {
	if fallbackStatuses[statusCode] {
		return true
	}
	lower := strings.ToLower(errorText)
	for _, pattern := range fallbackErrorPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}
