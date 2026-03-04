package main

import (
	"os"
	"strings"
)

type Config struct {
	Interpreter string
	Querent     string
	Provider    string
	Model       string
	BaseURL     string
	APIKey      string
}

// LoadConfig reads ./tarot.md and parses it into a Config.
// Returns nil (not an error) if the file doesn't exist.
func LoadConfig() (*Config, error) {
	data, err := os.ReadFile("tarot.md")
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
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
