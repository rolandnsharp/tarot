package main

import (
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Interpreter string
	Querent     string
	Provider    string
	Model       string
	BaseURL     string
	APIKey      string
	Deck        string // deck name (e.g. "cyberpunk"); empty = built-in classic
}

const defaultConfig = `## Interpreter

## Querent
The person sitting at this terminal, whoever they may be.

## Deck

## Connection
provider: ollama
model: qwen3:0.6b
base_url: http://localhost:11434/v1
api_key: unused
`

// configDir returns ~/.config/tarot, creating it if needed.
func configDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(dir, "tarot")
}

// LoadConfig searches for tarot.md in the current directory, then
// ~/.config/tarot/. If neither exists, creates a default at
// ~/.config/tarot/tarot.md and returns that.
func LoadConfig() (*Config, error) {
	// Search order: local, then config dir
	paths := []string{
		"tarot.md",
		filepath.Join(configDir(), "tarot.md"),
	}

	var data []byte
	var found bool
	for _, p := range paths {
		d, err := os.ReadFile(p)
		if err == nil {
			data = d
			found = true
			break
		}
	}

	if !found {
		// Create default config
		dir := configDir()
		os.MkdirAll(dir, 0o755)
		p := filepath.Join(dir, "tarot.md")
		if err := os.WriteFile(p, []byte(defaultConfig), 0o644); err != nil {
			return nil, nil // silently fall back to no config
		}
		data = []byte(defaultConfig)
	}

	cfg := &Config{}
	sections := strings.Split(string(data), "## ")

	for _, section := range sections {
		if section == "" {
			continue
		}
		newline := strings.IndexByte(section, '\n')
		if newline < 0 {
			continue
		}
		heading := strings.TrimSpace(section[:newline])
		body := strings.TrimSpace(section[newline+1:])

		switch heading {
		case "Interpreter":
			cfg.Interpreter = body
		case "Querent":
			cfg.Querent = body
		case "Deck":
			cfg.Deck = body
		case "Connection":
			for _, line := range strings.Split(body, "\n") {
				key, val, ok := strings.Cut(line, ":")
				if !ok {
					continue
				}
				key = strings.TrimSpace(key)
				val = strings.TrimSpace(val)
				switch key {
				case "provider":
					cfg.Provider = val
				case "model":
					cfg.Model = val
				case "base_url":
					cfg.BaseURL = val
				case "api_key":
					cfg.APIKey = val
				}
			}
		}
	}

	return cfg, nil
}
