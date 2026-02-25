package config

import (
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	// Server
	Port string
	Host string

	// Auth
	JWTSecret       string
	InitialPassword string

	// Storage
	DataDir string

	// API security
	APIKeySecret  string
	MachineIDSalt string
	RequireAPIKey bool

	// Logging
	EnableRequestLogs bool

	// URLs
	BaseURL  string
	CloudURL string

	// Proxy
	HTTPProxy  string
	HTTPSProxy string
	AllProxy   string
	NoProxy    string
}

func Load() *Config {
	dataDir := getenv("DATA_DIR", defaultDataDir())

	return &Config{
		Port:              getenv("PORT", "20128"),
		Host:              getenv("HOSTNAME", "0.0.0.0"),
		JWTSecret:         getenv("JWT_SECRET", "gorouter-default-secret-change-me"),
		InitialPassword:   getenv("INITIAL_PASSWORD", "123456"),
		DataDir:           dataDir,
		APIKeySecret:      getenv("API_KEY_SECRET", "endpoint-proxy-api-key-secret"),
		MachineIDSalt:     getenv("MACHINE_ID_SALT", "endpoint-proxy-salt"),
		RequireAPIKey:     getenvBool("REQUIRE_API_KEY", false),
		EnableRequestLogs: getenvBool("ENABLE_REQUEST_LOGS", false),
		BaseURL:           getenv("BASE_URL", getenv("NEXT_PUBLIC_BASE_URL", "http://localhost:20128")),
		CloudURL:          getenv("CLOUD_URL", getenv("NEXT_PUBLIC_CLOUD_URL", "https://gorouter.com")),
		HTTPProxy:         getenv("HTTP_PROXY", getenv("http_proxy", "")),
		HTTPSProxy:        getenv("HTTPS_PROXY", getenv("https_proxy", "")),
		AllProxy:          getenv("ALL_PROXY", getenv("all_proxy", "")),
		NoProxy:           getenv("NO_PROXY", getenv("no_proxy", "")),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".gorouter"
	}
	return filepath.Join(home, ".gorouter")
}
