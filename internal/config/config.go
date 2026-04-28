package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	APIKey         string `json:"api_key,omitempty"`
	DefaultMode    string `json:"default_mode,omitempty"`
	DefaultVoice   string `json:"default_voice,omitempty"`
	DefaultBotName string `json:"default_bot_name,omitempty"`
	TriggerWords   string `json:"trigger_words,omitempty"`
	Context        string `json:"context,omitempty"`
}

func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".agentcall", "config.json"), nil
}

func Load() (Config, error) {
	path, err := DefaultPath()
	if err != nil {
		return Config{}, err
	}
	return LoadFrom(path)
}

func LoadFrom(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Save(cfg Config) error {
	path, err := DefaultPath()
	if err != nil {
		return err
	}
	return SaveTo(cfg, path)
}

func SaveTo(cfg Config, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
