package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Tokens holds OAuth tokens for all providers.
type Tokens struct {
	Gemini *GeminiTokens `json:"gemini,omitempty"`
}

// GeminiTokens holds Google OAuth tokens for the Gemini API.
type GeminiTokens struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	Expiry       time.Time `json:"expiry"`
	Email        string    `json:"email"`
}

// DefaultTokensPath returns ~/.agentcall/tokens.json.
func DefaultTokensPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".agentcall", "tokens.json"), nil
}

// LoadTokens reads tokens from path. Returns empty Tokens (no error) if the
// file does not exist — treated as "not signed in".
func LoadTokens(path string) (Tokens, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Tokens{}, nil
	}
	if err != nil {
		return Tokens{}, err
	}
	var t Tokens
	if err := json.Unmarshal(data, &t); err != nil {
		return Tokens{}, err
	}
	return t, nil
}

// SaveTokens writes tokens to path (mode 0600), creating parent dirs as needed.
func SaveTokens(t Tokens, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
