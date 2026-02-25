package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"os"
)

// GetMachineID returns a stable 16-hex-char identifier derived from
// HMAC-SHA256(hostname + dataDir, salt).
func GetMachineID(salt, dataDir string) string {
	hostname, _ := os.Hostname()
	mac := hmac.New(sha256.New, []byte(salt))
	mac.Write([]byte(hostname + dataDir))
	return hex.EncodeToString(mac.Sum(nil))[:16]
}
