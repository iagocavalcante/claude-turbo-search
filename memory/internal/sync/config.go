package sync

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Remote string `json:"remote"`
	Token  string `json:"token"`
	// Name overrides the auto-derived "org/repo" label when pushing.
	// When set, the server treats it as a manual rename and won't let
	// future auto-pushes (with no override) overwrite it.
	Name string `json:"name,omitempty"`
}

// DefaultPath returns the path to the user-level config file.
// Overridable via CLAUDE_TURBO_SEARCH_CONFIG.
func DefaultPath() (string, error) {
	if p := os.Getenv("CLAUDE_TURBO_SEARCH_CONFIG"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "claude-turbo-search", "config.json"), nil
}

// Load reads the config at path. A missing file returns an empty Config (not an error).
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

// Save writes cfg to path with mode 0600 and creates parent dirs as needed.
func Save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// Clear removes the config file. Missing file is not an error.
func Clear(path string) error {
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// MaskToken returns a redacted version of the token for display.
func MaskToken(tok string) string {
	tok = strings.TrimSpace(tok)
	if tok == "" {
		return ""
	}
	if len(tok) <= 8 {
		return strings.Repeat("*", len(tok))
	}
	return tok[:4] + strings.Repeat("*", len(tok)-8) + tok[len(tok)-4:]
}
