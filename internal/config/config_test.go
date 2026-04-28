package config_test

import (
	"path/filepath"
	"testing"

	"agentcall-desktop/internal/config"
)

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := config.Config{
		APIKey:         "ak_ac_testkey",
		DefaultMode:    "audio",
		DefaultVoice:   "voice.heart",
		DefaultBotName: "Juno",
		TriggerWords:   "juno,june",
		Context:        "You are a meeting assistant.",
	}

	if err := config.SaveTo(cfg, path); err != nil {
		t.Fatalf("SaveTo error: %v", err)
	}

	loaded, err := config.LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom error: %v", err)
	}

	if loaded.APIKey != cfg.APIKey {
		t.Errorf("APIKey: got %q want %q", loaded.APIKey, cfg.APIKey)
	}
	if loaded.TriggerWords != cfg.TriggerWords {
		t.Errorf("TriggerWords: got %q want %q", loaded.TriggerWords, cfg.TriggerWords)
	}
}

func TestLoadMissing(t *testing.T) {
	_, err := config.LoadFrom("/nonexistent/path/config.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestDefaultConfigPath(t *testing.T) {
	path, err := config.DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath error: %v", err)
	}
	if path == "" {
		t.Error("expected non-empty path")
	}
}
