package usage

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Logger appends one-line summaries to log.txt.
type Logger struct {
	mu   sync.Mutex
	file string
}

// NewLogger creates a Logger that writes to dataDir/log.txt.
func NewLogger(dataDir string) *Logger {
	return &Logger{file: filepath.Join(dataDir, "log.txt")}
}

// Log appends a one-line request summary.
func (l *Logger) Log(e Entry) {
	line := fmt.Sprintf("[%s] %s %s/%s status=%d prompt=%d completion=%d cost=$%.6f dur=%dms\n",
		time.Now().UTC().Format(time.RFC3339),
		e.Endpoint,
		e.Provider,
		e.Model,
		e.StatusCode,
		e.PromptTokens,
		e.CompletionTokens,
		e.EstimatedCost,
		e.DurationMs,
	)

	l.mu.Lock()
	defer l.mu.Unlock()
	f, err := os.OpenFile(l.file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(line)
}
