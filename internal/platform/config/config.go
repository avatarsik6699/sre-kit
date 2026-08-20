// Package config loads typed settings from environment variables. It is the one place in the
// codebase allowed to call os.Getenv directly — every other package receives a *Config.
package config

import (
	"fmt"
	"os"
)

// Config holds every environment-derived setting the composition root needs to wire the app.
type Config struct {
	// Addr is the host:port the HTTP server listens on, e.g. ":8080".
	Addr string
	// DBPath is the filesystem path to the SQLite database file.
	DBPath string
	// SecretsPath is the filesystem path to the encrypted secrets store (secrets.enc.json).
	SecretsPath string
	// SecretsKey is the symmetric encryption key for the secrets store, read from
	// SRE_KIT_SECRETS_KEY. Required — the process refuses to start without it (§3, §6).
	SecretsKey string
	// AdaptersDir is the directory the adapter engine scans for installed adapter subprocesses.
	AdaptersDir string
}

const (
	envAddr        = "SRE_KIT_ADDR"
	envDBPath      = "SRE_KIT_DB_PATH"
	envSecretsPath = "SRE_KIT_SECRETS_PATH"
	envSecretsKey  = "SRE_KIT_SECRETS_KEY"
	envAdaptersDir = "SRE_KIT_ADAPTERS_DIR"
)

// Load reads Config from the process environment, applying defaults for everything except
// SecretsKey, and returns an error if a required variable is missing.
func Load() (Config, error) {
	cfg := Config{
		Addr:        getOr(envAddr, ":8080"),
		DBPath:      getOr(envDBPath, "./data/sre-kit.db"),
		SecretsPath: getOr(envSecretsPath, "./data/secrets.enc.json"),
		SecretsKey:  os.Getenv(envSecretsKey),
		AdaptersDir: getOr(envAdaptersDir, "./adapters"),
	}
	if cfg.SecretsKey == "" {
		return Config{}, fmt.Errorf("config: %s is required (secrets store encryption key)", envSecretsKey)
	}
	return cfg, nil
}

func getOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
