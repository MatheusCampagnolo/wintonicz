// Package config holds user preferences. It stores settings only; nothing in the
// file can add commands or run anything.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Config is the on-disk preference file.
type Config struct {
	// ReadOnly refuses every action in the CLI and TUI.
	ReadOnly bool `json:"readOnly"`
	// JSONDefault makes CLI commands print JSON without --json.
	JSONDefault bool `json:"jsonDefault"`
	// ActionLog writes a local log line for every action.
	ActionLog bool `json:"actionLog"`
	// DefaultArea is the TUI's opening area: home, inspect, network, or windows.
	DefaultArea string `json:"defaultArea"`
}

// Default returns the built-in preferences.
func Default() Config {
	return Config{ActionLog: true, DefaultArea: "home"}
}

var areas = []string{"home", "inspect", "network", "repair", "clean", "windows", "reports", "settings"}

// Path returns the preference file location, or "" when no per-user folder is known.
func Path() string {
	base := os.Getenv("APPDATA")
	if base == "" {
		return ""
	}
	return filepath.Join(base, "Wintonicz", "config.json")
}

// Load reads the file (missing means defaults) and applies environment overrides:
// WTZ_READ_ONLY=1 and WTZ_JSON=1.
func Load() (Config, error) {
	cfg := Default()
	var loadErr error
	if path := Path(); path != "" {
		data, err := os.ReadFile(path)
		switch {
		case err == nil:
			if err := json.Unmarshal(data, &cfg); err != nil {
				loadErr = fmt.Errorf("read %s: %w", path, err)
				cfg = Default()
			}
		case !errors.Is(err, os.ErrNotExist):
			loadErr = err
		}
	}
	if cfg.DefaultArea == "" {
		cfg.DefaultArea = "home"
	}
	if os.Getenv("WTZ_READ_ONLY") == "1" {
		cfg.ReadOnly = true
	}
	if os.Getenv("WTZ_JSON") == "1" {
		cfg.JSONDefault = true
	}
	return cfg, loadErr
}

// Save writes the preference file.
func Save(cfg Config) error {
	path := Path()
	if path == "" {
		return errors.New("no per-user settings folder is available (APPDATA is not set)")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

// Keys lists the settable keys.
func Keys() []string {
	keys := []string{"readOnly", "jsonDefault", "actionLog", "defaultArea"}
	sort.Strings(keys)
	return keys
}

// Get returns the value of a key as text.
func Get(cfg Config, key string) (string, error) {
	switch key {
	case "readOnly":
		return strconv.FormatBool(cfg.ReadOnly), nil
	case "jsonDefault":
		return strconv.FormatBool(cfg.JSONDefault), nil
	case "actionLog":
		return strconv.FormatBool(cfg.ActionLog), nil
	case "defaultArea":
		return cfg.DefaultArea, nil
	}
	return "", unknownKey(key)
}

// Set validates and applies a value.
func Set(cfg *Config, key, value string) error {
	setBool := func(target *bool) error {
		parsed, err := strconv.ParseBool(strings.TrimSpace(value))
		if err != nil {
			return fmt.Errorf("%s must be true or false", key)
		}
		*target = parsed
		return nil
	}
	switch key {
	case "readOnly":
		return setBool(&cfg.ReadOnly)
	case "jsonDefault":
		return setBool(&cfg.JSONDefault)
	case "actionLog":
		return setBool(&cfg.ActionLog)
	case "defaultArea":
		value = strings.ToLower(strings.TrimSpace(value))
		for _, area := range areas {
			if area == value {
				cfg.DefaultArea = value
				return nil
			}
		}
		return fmt.Errorf("defaultArea must be one of: %s", strings.Join(areas, ", "))
	}
	return unknownKey(key)
}

func unknownKey(key string) error {
	return fmt.Errorf("unknown setting %q; settings are: %s", key, strings.Join(Keys(), ", "))
}
