package auth_test

import (
	"path/filepath"
	"testing"
	"time"

	"agentcall-desktop/internal/auth"
)

func TestSaveAndLoadTokens(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens.json")

	expiry := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	tok := auth.Tokens{
		Gemini: &auth.GeminiTokens{
			AccessToken:  "ya29.test",
			RefreshToken: "1//test-refresh",
			Expiry:       expiry,
			Email:        "user@gmail.com",
		},
	}

	if err := auth.SaveTokens(tok, path); err != nil {
		t.Fatalf("SaveTokens: %v", err)
	}

	loaded, err := auth.LoadTokens(path)
	if err != nil {
		t.Fatalf("LoadTokens: %v", err)
	}

	if loaded.Gemini == nil {
		t.Fatal("Gemini tokens missing after load")
	}
	if loaded.Gemini.AccessToken != "ya29.test" {
		t.Errorf("AccessToken: got %q want %q", loaded.Gemini.AccessToken, "ya29.test")
	}
	if loaded.Gemini.Email != "user@gmail.com" {
		t.Errorf("Email: got %q", loaded.Gemini.Email)
	}
	if !loaded.Gemini.Expiry.Equal(expiry) {
		t.Errorf("Expiry: got %v want %v", loaded.Gemini.Expiry, expiry)
	}
}

func TestLoadMissingTokens(t *testing.T) {
	tok, err := auth.LoadTokens("/nonexistent/tokens.json")
	if err != nil {
		t.Fatalf("expected empty tokens, got error: %v", err)
	}
	if tok.Gemini != nil {
		t.Error("expected nil Gemini tokens for missing file")
	}
}

func TestSignOut(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens.json")

	tok := auth.Tokens{
		Gemini: &auth.GeminiTokens{AccessToken: "ya29.test", Email: "u@g.com"},
	}
	if err := auth.SaveTokens(tok, path); err != nil {
		t.Fatalf("SaveTokens (setup): %v", err)
	}

	cleared := auth.Tokens{}
	if err := auth.SaveTokens(cleared, path); err != nil {
		t.Fatalf("SaveTokens (clear): %v", err)
	}

	loaded, _ := auth.LoadTokens(path)
	if loaded.Gemini != nil {
		t.Error("expected Gemini to be nil after sign-out")
	}
}
