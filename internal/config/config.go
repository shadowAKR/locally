// Package config persists user settings to ~/.config/localagent/settings.json.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Settings is everything the user can tweak from the UI.
type Settings struct {
	OllamaURL   string  `json:"ollamaUrl"`
	Model       string  `json:"model"`
	AutoApprove bool    `json:"autoApprove"`
	Think       bool    `json:"think"`
	NumCtx      int     `json:"numCtx"`
	Temperature float64 `json:"temperature"`
}

// Default returns sensible defaults for a first run.
func Default() Settings {
	return Settings{
		OllamaURL:   "http://localhost:11434",
		Model:       "",
		AutoApprove: false,
		Think:       false,
		NumCtx:      16384,
		Temperature: 0.2,
	}
}

func path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "localagent", "settings.json"), nil
}

// Load reads settings from disk, falling back to defaults.
func Load() Settings {
	s := Default()
	p, err := path()
	if err != nil {
		return s
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return s
	}
	_ = json.Unmarshal(data, &s)
	if s.OllamaURL == "" {
		s.OllamaURL = Default().OllamaURL
	}
	if s.NumCtx <= 0 {
		s.NumCtx = Default().NumCtx
	}
	return s
}

// Save writes settings to disk.
func Save(s Settings) error {
	p, err := path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, p); err != nil {
		return errors.Join(err, os.Remove(tmp))
	}
	return nil
}
