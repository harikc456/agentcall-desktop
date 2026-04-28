# Gemini Brain Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a Gemini-powered LLM loop inside agentcall-desktop so the bot actively processes meeting transcripts and feeds responses to GetSun's TTS — with no Claude Code required.

**Architecture:** Three new packages (`internal/auth`, `internal/llm`, `internal/brain`) sit above existing plumbing. `auth` handles Google OAuth PKCE and token storage. `llm` is a pure Gemini REST client. `brain` runs a goroutine that consumes `transcript.final` WebSocket events, calls Gemini, and sends `voice.context_update` + `trigger.speak` commands back. `app.go` wires them together and exposes three new Wails-bound methods.

**Tech Stack:** Go 1.23, standard library (`crypto/rand`, `crypto/sha256`, `net/http`), `github.com/pkg/browser` (already in go.mod), Wails v2, Gemini REST API (`generativelanguage.googleapis.com/v1beta`).

---

## File Map

| Action | Path | Responsibility |
|--------|------|----------------|
| Create | `internal/auth/tokens.go` | Token struct, load/save `~/.agentcall/tokens.json` |
| Create | `internal/auth/tokens_test.go` | Tests for token persistence |
| Create | `internal/auth/google.go` | Google OAuth PKCE flow, access-token refresh |
| Create | `internal/auth/google_test.go` | Tests for PKCE helpers |
| Create | `internal/llm/gemini.go` | Gemini REST client, conversation history |
| Create | `internal/llm/gemini_test.go` | Tests for request/response marshaling |
| Create | `internal/brain/brain.go` | LLM loop goroutine |
| Create | `internal/brain/brain_test.go` | Tests for event filtering and command building |
| Modify | `internal/bridge/websocket.go` | Add `SendJSON(v any) error` |
| Modify | `internal/bridge/types.go` | Add `ContextUpdateCmd`, `TriggerSpeakCmd` |
| Modify | `internal/bridge/types_test.go` | Tests for new command types |
| Modify | `app.go` | Wire auth+brain, add three Wails methods |
| Modify | `frontend/index.html` | Gemini auth section in settings, chip in call screen |
| Modify | `frontend/src/main.js` | Auth button handlers, Wails event listeners |

---

## Task 1: Bridge — SendJSON + voice command types

Voice intelligence commands use `{"type": "voice.context_update", ...}` — different field name than the existing `Command` struct (`"command"`). Add a `SendJSON` escape hatch and typed structs.

**Files:**
- Modify: `internal/bridge/websocket.go`
- Modify: `internal/bridge/types.go`
- Modify: `internal/bridge/types_test.go`

- [ ] **Step 1: Write failing tests for new command types**

Open `internal/bridge/types_test.go` and add after the existing tests:

```go
func TestContextUpdateCmdJSON(t *testing.T) {
	cmd := bridge.ContextUpdateCmd{
		Type: "voice.context_update",
		Text: "The revenue was $2.4M.",
	}
	data, err := json.Marshal(cmd)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var m map[string]string
	json.Unmarshal(data, &m)
	if m["type"] != "voice.context_update" {
		t.Errorf("type: got %q want %q", m["type"], "voice.context_update")
	}
	if m["text"] != "The revenue was $2.4M." {
		t.Errorf("text: got %q", m["text"])
	}
}

func TestTriggerSpeakCmdJSON(t *testing.T) {
	cmd := bridge.TriggerSpeakCmd{
		Type:    "trigger.speak",
		Text:    "What is the revenue?",
		Speaker: "Alice",
	}
	data, err := json.Marshal(cmd)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var m map[string]string
	json.Unmarshal(data, &m)
	if m["type"] != "trigger.speak" {
		t.Errorf("type: got %q", m["type"])
	}
	if m["speaker"] != "Alice" {
		t.Errorf("speaker: got %q", m["speaker"])
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/harikrishnan-c/projects/agent-call/agentcall-desktop
go test ./internal/bridge/... -run "TestContextUpdate|TestTriggerSpeak" -v
```

Expected: compile error — `bridge.ContextUpdateCmd` and `bridge.TriggerSpeakCmd` undefined.

- [ ] **Step 3: Add types to `internal/bridge/types.go`**

Append to the end of `internal/bridge/types.go`:

```go
// ContextUpdateCmd replaces GetSun's context scratchpad (max 4000 chars).
// Uses "type" field (direct WebSocket protocol, not bridge.py subprocess protocol).
type ContextUpdateCmd struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// TriggerSpeakCmd forces GetSun to respond as if asked the given question.
type TriggerSpeakCmd struct {
	Type    string `json:"type"`
	Text    string `json:"text"`
	Speaker string `json:"speaker,omitempty"`
}
```

- [ ] **Step 4: Add `SendJSON` to `internal/bridge/websocket.go`**

Append after the `SendCommand` method:

```go
// SendJSON writes any JSON-serialisable value to the WebSocket.
// Use for voice intelligence commands that require the "type" field
// rather than the "command" field used by SendCommand.
func (b *Bridge) SendJSON(v any) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.conn.WriteJSON(v)
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/bridge/... -v
```

Expected: all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/bridge/websocket.go internal/bridge/types.go internal/bridge/types_test.go
git commit -m "feat(bridge): add SendJSON and voice intelligence command types"
```

---

## Task 2: Token storage (`internal/auth/tokens.go`)

Persist Google OAuth tokens to `~/.agentcall/tokens.json` (separate from config).

**Files:**
- Create: `internal/auth/tokens.go`
- Create: `internal/auth/tokens_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/auth/tokens_test.go`:

```go
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
	auth.SaveTokens(tok, path)

	cleared := auth.Tokens{}
	if err := auth.SaveTokens(cleared, path); err != nil {
		t.Fatalf("SaveTokens (clear): %v", err)
	}

	loaded, _ := auth.LoadTokens(path)
	if loaded.Gemini != nil {
		t.Error("expected Gemini to be nil after sign-out")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/auth/... -v
```

Expected: compile error — package `auth` not found.

- [ ] **Step 3: Create `internal/auth/tokens.go`**

```go
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
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/auth/... -v
```

Expected: all three tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/tokens.go internal/auth/tokens_test.go
git commit -m "feat(auth): token storage for Google OAuth credentials"
```

---

## Task 3: Google OAuth PKCE (`internal/auth/google.go`)

Implements the "Sign in with Google" flow: PKCE, local callback server, token exchange, token refresh.

**Files:**
- Create: `internal/auth/google.go`
- Create: `internal/auth/google_test.go`

**Prerequisite:** Before this code can actually authenticate users, register the app in Google Cloud Console:
1. Go to https://console.cloud.google.com → APIs & Services → Credentials
2. Create OAuth 2.0 Client ID → Application type: **Desktop app**
3. Authorized redirect URIs: `http://localhost` (covers all ports)
4. Enable the **Generative Language API** in APIs & Services → Library
5. Copy the `client_id` into the `googleClientID` constant below

- [ ] **Step 1: Write failing tests for PKCE helpers**

Create `internal/auth/google_test.go`:

```go
package auth_test

import (
	"testing"

	"agentcall-desktop/internal/auth"
)

func TestGenerateVerifier(t *testing.T) {
	v1, err := auth.GenerateVerifier()
	if err != nil {
		t.Fatalf("GenerateVerifier: %v", err)
	}
	v2, _ := auth.GenerateVerifier()
	if v1 == v2 {
		t.Error("expected different verifiers each call")
	}
	if len(v1) < 40 {
		t.Errorf("verifier too short: %q", v1)
	}
}

func TestComputeChallenge(t *testing.T) {
	// RFC 7636 Appendix B test vector
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	want := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	got := auth.ComputeChallenge(verifier)
	if got != want {
		t.Errorf("ComputeChallenge: got %q want %q", got, want)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/auth/... -run "TestGenerate|TestCompute" -v
```

Expected: compile error — `auth.GenerateVerifier` and `auth.ComputeChallenge` undefined.

- [ ] **Step 3: Create `internal/auth/google.go`**

```go
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pkg/browser"
)

// googleClientID is the OAuth 2.0 client ID registered in Google Cloud Console
// as a Desktop application. Set this before shipping.
const googleClientID = "YOUR_GOOGLE_CLIENT_ID.apps.googleusercontent.com"

const (
	googleAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL = "https://oauth2.googleapis.com/token"
	geminiScope    = "https://www.googleapis.com/auth/generative-language"
	emailScope     = "email"
)

// GenerateVerifier creates a random PKCE code verifier (base64url, no padding).
func GenerateVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// ComputeChallenge returns the S256 PKCE code challenge for a verifier.
func ComputeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// Provider manages Google OAuth tokens for a single user.
type Provider struct {
	tokensPath string
}

// NewProvider creates a Provider that persists tokens to tokensPath.
func NewProvider(tokensPath string) *Provider {
	return &Provider{tokensPath: tokensPath}
}

// IsAuthenticated returns true if valid tokens are stored.
func (p *Provider) IsAuthenticated() bool {
	tok, err := LoadTokens(p.tokensPath)
	return err == nil && tok.Gemini != nil && tok.Gemini.RefreshToken != ""
}

// Email returns the stored Google account email, or empty string.
func (p *Provider) Email() string {
	tok, err := LoadTokens(p.tokensPath)
	if err != nil || tok.Gemini == nil {
		return ""
	}
	return tok.Gemini.Email
}

// AccessToken returns a valid access token, refreshing if within 60s of expiry.
func (p *Provider) AccessToken(ctx context.Context) (string, error) {
	tok, err := LoadTokens(p.tokensPath)
	if err != nil || tok.Gemini == nil {
		return "", fmt.Errorf("not authenticated")
	}
	g := tok.Gemini
	if time.Until(g.Expiry) > 60*time.Second {
		return g.AccessToken, nil
	}
	return p.refresh(ctx, tok)
}

// StartOAuth opens the browser and runs the PKCE flow, blocking until done.
func (p *Provider) StartOAuth(ctx context.Context) error {
	verifier, err := GenerateVerifier()
	if err != nil {
		return fmt.Errorf("pkce verifier: %w", err)
	}
	challenge := ComputeChallenge(verifier)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("local listener: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://localhost:%d/callback", port)

	authURL := buildAuthURL(redirectURI, challenge)
	if err := browser.OpenURL(authURL); err != nil {
		return fmt.Errorf("open browser: %w", err)
	}

	codeCh := make(chan string, 1)
	srv := &http.Server{}
	srv.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			return
		}
		fmt.Fprintln(w, "<html><body><h2>Signed in! You can close this tab.</h2></body></html>")
		codeCh <- code
	})
	go srv.Serve(ln)
	defer srv.Close()

	select {
	case code := <-codeCh:
		return p.exchange(ctx, code, verifier, redirectURI)
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Minute):
		return fmt.Errorf("OAuth timeout — browser not completed within 5 minutes")
	}
}

// SignOut deletes stored Gemini tokens.
func (p *Provider) SignOut() error {
	tok, err := LoadTokens(p.tokensPath)
	if err != nil {
		return err
	}
	tok.Gemini = nil
	return SaveTokens(tok, p.tokensPath)
}

// --- internal helpers ---

func buildAuthURL(redirectURI, challenge string) string {
	v := url.Values{}
	v.Set("client_id", googleClientID)
	v.Set("redirect_uri", redirectURI)
	v.Set("response_type", "code")
	v.Set("scope", geminiScope+" "+emailScope)
	v.Set("code_challenge", challenge)
	v.Set("code_challenge_method", "S256")
	v.Set("access_type", "offline")
	v.Set("prompt", "consent")
	return googleAuthURL + "?" + v.Encode()
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Email        string `json:"email"`
}

func (p *Provider) exchange(ctx context.Context, code, verifier, redirectURI string) error {
	body := url.Values{}
	body.Set("client_id", googleClientID)
	body.Set("code", code)
	body.Set("code_verifier", verifier)
	body.Set("grant_type", "authorization_code")
	body.Set("redirect_uri", redirectURI)

	tr, err := postToken(ctx, body)
	if err != nil {
		return err
	}

	email := tr.Email
	if email == "" {
		email, _ = fetchEmail(ctx, tr.AccessToken)
	}

	tok, _ := LoadTokens(p.tokensPath)
	tok.Gemini = &GeminiTokens{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		Expiry:       time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second),
		Email:        email,
	}
	return SaveTokens(tok, p.tokensPath)
}

func (p *Provider) refresh(ctx context.Context, tok Tokens) (string, error) {
	body := url.Values{}
	body.Set("client_id", googleClientID)
	body.Set("grant_type", "refresh_token")
	body.Set("refresh_token", tok.Gemini.RefreshToken)

	tr, err := postToken(ctx, body)
	if err != nil {
		return "", fmt.Errorf("token refresh: %w", err)
	}

	tok.Gemini.AccessToken = tr.AccessToken
	tok.Gemini.Expiry = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	if err := SaveTokens(tok, p.tokensPath); err != nil {
		return "", err
	}
	return tr.AccessToken, nil
}

func postToken(ctx context.Context, body url.Values) (*tokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleTokenURL,
		strings.NewReader(body.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token endpoint %d: %s", resp.StatusCode, b)
	}

	var tr tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return nil, err
	}
	return &tr, nil
}

func fetchEmail(ctx context.Context, accessToken string) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://www.googleapis.com/oauth2/v3/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var info struct {
		Email string `json:"email"`
	}
	json.NewDecoder(resp.Body).Decode(&info)
	return info.Email, nil
}
```

- [ ] **Step 4: Run PKCE tests to verify they pass**

```bash
go test ./internal/auth/... -run "TestGenerate|TestCompute" -v
```

Expected: both tests PASS.

- [ ] **Step 5: Run full auth tests**

```bash
go test ./internal/auth/... -v
```

Expected: all five tests PASS (`TestSaveAndLoad`, `TestLoadMissing`, `TestSignOut`, `TestGenerateVerifier`, `TestComputeChallenge`).

- [ ] **Step 6: Commit**

```bash
git add internal/auth/google.go internal/auth/google_test.go
git commit -m "feat(auth): Google OAuth PKCE flow and token refresh"
```

---

## Task 4: Gemini client (`internal/llm/gemini.go`)

Stateful Gemini REST client that maintains conversation history and calls `gemini-2.0-flash`.

**Files:**
- Create: `internal/llm/gemini.go`
- Create: `internal/llm/gemini_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/llm/gemini_test.go`:

```go
package llm_test

import (
	"encoding/json"
	"testing"

	"agentcall-desktop/internal/llm"
)

func TestNewClientHasSystemPrompt(t *testing.T) {
	c := llm.NewClient("Juno", "You help with Q3 revenue.")
	req := c.BuildRequest("What is the revenue?")

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]interface{}
	json.Unmarshal(data, &m)

	si, ok := m["system_instruction"].(map[string]interface{})
	if !ok {
		t.Fatal("system_instruction missing")
	}
	parts, _ := si["parts"].([]interface{})
	if len(parts) == 0 {
		t.Fatal("system_instruction.parts empty")
	}
	text := parts[0].(map[string]interface{})["text"].(string)
	if text == "" {
		t.Error("system prompt text is empty")
	}
}

func TestBuildRequestContents(t *testing.T) {
	c := llm.NewClient("Juno", "")
	req := c.BuildRequest("Hello Juno")

	if len(req.Contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(req.Contents))
	}
	if req.Contents[0].Role != "user" {
		t.Errorf("role: got %q want user", req.Contents[0].Role)
	}
	if req.Contents[0].Parts[0].Text != "Hello Juno" {
		t.Errorf("text: got %q", req.Contents[0].Parts[0].Text)
	}
}

func TestRecordTurnGrowsHistory(t *testing.T) {
	c := llm.NewClient("Juno", "")
	c.RecordTurn("Hello Juno", "Hi there!")
	req := c.BuildRequest("How are you?")

	// history: user "Hello Juno", model "Hi there!", user "How are you?" = 3
	if len(req.Contents) != 3 {
		t.Fatalf("expected 3 contents, got %d", len(req.Contents))
	}
}

func TestHistoryCapAt20Turns(t *testing.T) {
	c := llm.NewClient("Juno", "")
	for i := 0; i < 12; i++ {
		c.RecordTurn("question", "answer")
	}
	req := c.BuildRequest("new question")
	// 20 history entries (12 pairs = 24 messages, capped at 20) + 1 new = 21 max
	if len(req.Contents) > 21 {
		t.Errorf("history not capped: got %d contents", len(req.Contents))
	}
}

func TestParseResponse(t *testing.T) {
	raw := `{
		"candidates": [{
			"content": {
				"parts": [{"text": "The revenue was $2.4M."}],
				"role": "model"
			}
		}]
	}`
	text, err := llm.ParseResponse([]byte(raw))
	if err != nil {
		t.Fatalf("ParseResponse: %v", err)
	}
	if text != "The revenue was $2.4M." {
		t.Errorf("text: got %q", text)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/llm/... -v
```

Expected: compile error — package `llm` not found.

- [ ] **Step 3: Create `internal/llm/gemini.go`**

```go
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
)

const (
	geminiEndpoint = "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent"
	maxHistoryTurns = 20
)

// Part is a single text part in a Gemini message.
type Part struct {
	Text string `json:"text"`
}

// Content is a single turn in the conversation.
type Content struct {
	Role  string `json:"role"`
	Parts []Part `json:"parts"`
}

// GenerateRequest is the body for Gemini generateContent.
type GenerateRequest struct {
	SystemInstruction *Content  `json:"system_instruction,omitempty"`
	Contents          []Content `json:"contents"`
}

type generateResponse struct {
	Candidates []struct {
		Content Content `json:"content"`
	} `json:"candidates"`
}

// Client calls the Gemini API and maintains conversation history.
type Client struct {
	botName  string
	sysPrompt string
	history  []Content
	mu       sync.Mutex
}

// NewClient creates a Gemini client. botName and userContext are woven into the
// system prompt. userContext may be empty.
func NewClient(botName, userContext string) *Client {
	prompt := fmt.Sprintf(
		"You are %s, an AI meeting assistant. Respond conversationally and concisely (2-3 sentences max).",
		botName,
	)
	if userContext != "" {
		prompt += "\n\n" + userContext
	}
	return &Client{botName: botName, sysPrompt: prompt}
}

// BuildRequest constructs a GenerateRequest from history + the new transcript.
// Exposed for testing; callers normally use Chat().
func (c *Client) BuildRequest(transcript string) GenerateRequest {
	c.mu.Lock()
	defer c.mu.Unlock()

	history := c.history
	if len(history) > maxHistoryTurns {
		history = history[len(history)-maxHistoryTurns:]
	}

	contents := make([]Content, len(history)+1)
	copy(contents, history)
	contents[len(history)] = Content{
		Role:  "user",
		Parts: []Part{{Text: transcript}},
	}

	return GenerateRequest{
		SystemInstruction: &Content{Parts: []Part{{Text: c.sysPrompt}}},
		Contents:          contents,
	}
}

// RecordTurn appends a user/model pair to conversation history.
// Call after a successful Chat() to keep history current.
func (c *Client) RecordTurn(userText, modelText string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.history = append(c.history,
		Content{Role: "user", Parts: []Part{{Text: userText}}},
		Content{Role: "model", Parts: []Part{{Text: modelText}}},
	)
}

// ParseResponse extracts the text from a raw Gemini API response body.
// Exposed for testing.
func ParseResponse(body []byte) (string, error) {
	var resp generateResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("decode gemini response: %w", err)
	}
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response from Gemini")
	}
	return resp.Candidates[0].Content.Parts[0].Text, nil
}

// Chat sends transcript to Gemini and returns the model's response text.
// accessToken must be a valid Google OAuth access token with the
// generative-language scope.
func (c *Client) Chat(ctx context.Context, accessToken, transcript string) (string, error) {
	req := c.BuildRequest(transcript)

	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, geminiEndpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Authorization", "Bearer "+accessToken)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("gemini request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gemini %d: %s", resp.StatusCode, respBody)
	}

	text, err := ParseResponse(respBody)
	if err != nil {
		return "", err
	}

	c.RecordTurn(transcript, text)
	return text, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/llm/... -v
```

Expected: all five tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/llm/gemini.go internal/llm/gemini_test.go
git commit -m "feat(llm): Gemini REST client with conversation history"
```

---

## Task 5: Brain goroutine (`internal/brain/brain.go`)

The event loop: receives `transcript.final` from the call, calls Gemini, sends `voice.context_update` + `trigger.speak` back.

**Files:**
- Create: `internal/brain/brain.go`
- Create: `internal/brain/brain_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/brain/brain_test.go`:

```go
package brain_test

import (
	"testing"

	"agentcall-desktop/internal/brain"
	"agentcall-desktop/internal/bridge"
)

func TestShouldProcessEvent(t *testing.T) {
	tests := []struct {
		name    string
		event   bridge.Event
		botName string
		want    bool
	}{
		{
			name:    "transcript from human",
			event:   bridge.Event{Type: "transcript.final", Text: "Hello Juno", Speaker: &bridge.Speaker{Name: "Alice"}},
			botName: "Juno",
			want:    true,
		},
		{
			name:    "transcript from bot (feedback loop guard)",
			event:   bridge.Event{Type: "transcript.final", Text: "Hello", Speaker: &bridge.Speaker{Name: "Juno"}},
			botName: "Juno",
			want:    false,
		},
		{
			name:    "empty text — skip",
			event:   bridge.Event{Type: "transcript.final", Text: "", Speaker: &bridge.Speaker{Name: "Alice"}},
			botName: "Juno",
			want:    false,
		},
		{
			name:    "wrong event type",
			event:   bridge.Event{EventType: "call.bot_ready"},
			botName: "Juno",
			want:    false,
		},
		{
			name:    "no speaker field",
			event:   bridge.Event{Type: "transcript.final", Text: "Hello"},
			botName: "Juno",
			want:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := brain.ShouldProcess(tc.event, tc.botName)
			if got != tc.want {
				t.Errorf("ShouldProcess = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBuildCommands(t *testing.T) {
	ctxCmd, trigCmd := brain.BuildCommands("Alice", "What is revenue?", "Revenue was $2.4M.")

	if ctxCmd.Type != "voice.context_update" {
		t.Errorf("context cmd type: got %q", ctxCmd.Type)
	}
	if ctxCmd.Text != "Revenue was $2.4M." {
		t.Errorf("context cmd text: got %q", ctxCmd.Text)
	}
	if trigCmd.Type != "trigger.speak" {
		t.Errorf("trigger cmd type: got %q", trigCmd.Type)
	}
	if trigCmd.Text != "What is revenue?" {
		t.Errorf("trigger cmd text: got %q", trigCmd.Text)
	}
	if trigCmd.Speaker != "Alice" {
		t.Errorf("trigger cmd speaker: got %q", trigCmd.Speaker)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/brain/... -v
```

Expected: compile error — package `brain` not found.

- [ ] **Step 3: Create `internal/brain/brain.go`**

```go
package brain

import (
	"context"
	"log"

	"agentcall-desktop/internal/bridge"
	"agentcall-desktop/internal/llm"
)

// Sender is the subset of bridge.Bridge used by Brain — allows easy testing.
type Sender interface {
	SendJSON(v any) error
}

// TokenProvider returns a valid OAuth access token, refreshing as needed.
type TokenProvider interface {
	AccessToken(ctx context.Context) (string, error)
}

// Brain runs the LLM loop: transcript.final → Gemini → GetSun voice commands.
type Brain struct {
	ws      Sender
	llm     *llm.Client
	tokens  TokenProvider
	botName string
	events  chan bridge.Event
	cancel  context.CancelFunc
}

// New creates a Brain. Call Start to begin processing.
func New(ws Sender, llmClient *llm.Client, tokens TokenProvider, botName string) *Brain {
	return &Brain{
		ws:      ws,
		llm:     llmClient,
		tokens:  tokens,
		botName: botName,
		events:  make(chan bridge.Event, 32),
	}
}

// Start begins the event loop in a goroutine.
func (b *Brain) Start(ctx context.Context) {
	ctx, b.cancel = context.WithCancel(ctx)
	go b.run(ctx)
}

// Stop cancels the brain goroutine.
func (b *Brain) Stop() {
	if b.cancel != nil {
		b.cancel()
	}
}

// Feed delivers a WebSocket event to the brain for processing.
// Non-blocking: events are dropped if the internal buffer is full.
func (b *Brain) Feed(event bridge.Event) {
	select {
	case b.events <- event:
	default:
	}
}

func (b *Brain) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-b.events:
			if !ok {
				return
			}
			if !ShouldProcess(event, b.botName) {
				continue
			}
			b.process(ctx, event)
		}
	}
}

func (b *Brain) process(ctx context.Context, event bridge.Event) {
	accessToken, err := b.tokens.AccessToken(ctx)
	if err != nil {
		log.Printf("brain: get access token: %v", err)
		return
	}

	response, err := b.llm.Chat(ctx, accessToken, event.Text)
	if err != nil {
		log.Printf("brain: gemini chat: %v", err)
		return
	}

	speakerName := ""
	if event.Speaker != nil {
		speakerName = event.Speaker.Name
	}

	ctxCmd, trigCmd := BuildCommands(speakerName, event.Text, response)

	if err := b.ws.SendJSON(ctxCmd); err != nil {
		log.Printf("brain: send context_update: %v", err)
		return
	}
	if err := b.ws.SendJSON(trigCmd); err != nil {
		log.Printf("brain: send trigger.speak: %v", err)
	}
}

// ShouldProcess returns true if the event should be sent to the LLM.
// Exported for testing.
func ShouldProcess(event bridge.Event, botName string) bool {
	if event.Normalize() != "transcript.final" {
		return false
	}
	if event.Text == "" {
		return false
	}
	if event.Speaker != nil && event.Speaker.Name == botName {
		return false
	}
	return true
}

// BuildCommands constructs the two WebSocket commands to send after an LLM response.
// Exported for testing.
func BuildCommands(speakerName, transcript, response string) (bridge.ContextUpdateCmd, bridge.TriggerSpeakCmd) {
	return bridge.ContextUpdateCmd{
			Type: "voice.context_update",
			Text: response,
		}, bridge.TriggerSpeakCmd{
			Type:    "trigger.speak",
			Text:    transcript,
			Speaker: speakerName,
		}
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/brain/... -v
```

Expected: all tests PASS.

- [ ] **Step 5: Run all tests**

```bash
go test ./... -v
```

Expected: all packages PASS with no compile errors.

- [ ] **Step 6: Commit**

```bash
git add internal/brain/brain.go internal/brain/brain_test.go
git commit -m "feat(brain): LLM event loop — transcript.final → Gemini → GetSun commands"
```

---

## Task 6: Wire into `app.go`

Connect auth, llm, and brain into the Wails app. Add three new bound methods. Modify `JoinMeeting`, `LeaveCall`, `cleanup`, and `forwardEvents`.

**Files:**
- Modify: `app.go`

- [ ] **Step 1: Replace `app.go` with the wired-up version**

Replace the entire contents of `app.go`:

```go
package main

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"agentcall-desktop/internal/auth"
	"agentcall-desktop/internal/brain"
	"agentcall-desktop/internal/bridge"
	"agentcall-desktop/internal/config"
	"agentcall-desktop/internal/llm"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails application struct. All exported methods are bound to the frontend.
type App struct {
	ctx    context.Context
	mu     sync.Mutex
	client *bridge.Client
	ws     *bridge.Bridge
	callID string
	br     *brain.Brain
	auth   *auth.Provider
}

// GeminiStatus is returned to the frontend.
type GeminiStatus struct {
	Authenticated bool   `json:"authenticated"`
	Email         string `json:"email"`
}

func NewApp() *App {
	tokPath, _ := auth.DefaultTokensPath()
	return &App{
		auth: auth.NewProvider(tokPath),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) beforeClose(ctx context.Context) bool {
	a.cleanup()
	return false
}

func (a *App) cleanup() {
	a.mu.Lock()
	ws := a.ws
	callID := a.callID
	client := a.client
	br := a.br
	a.ws = nil
	a.callID = ""
	a.br = nil
	a.mu.Unlock()

	if br != nil {
		br.Stop()
	}
	if ws != nil {
		ws.Close()
	}
	if client != nil && callID != "" {
		_ = client.DeleteCall(callID)
	}
}

// GetConfig returns the saved config (or empty config if none exists).
func (a *App) GetConfig() config.Config {
	cfg, err := config.Load()
	if err != nil {
		return config.Config{}
	}
	return cfg
}

// SaveConfig writes cfg to ~/.agentcall/config.json.
func (a *App) SaveConfig(cfg config.Config) error {
	return config.Save(cfg)
}

// GetGeminiStatus returns whether the user is signed in with Google.
func (a *App) GetGeminiStatus() GeminiStatus {
	return GeminiStatus{
		Authenticated: a.auth.IsAuthenticated(),
		Email:         a.auth.Email(),
	}
}

// StartGeminiAuth opens the browser and runs the Google OAuth PKCE flow.
// Emits "auth.gemini_ready" on success or "auth.gemini_error" on failure.
func (a *App) StartGeminiAuth() error {
	go func() {
		err := a.auth.StartOAuth(a.ctx)
		if err != nil {
			runtime.EventsEmit(a.ctx, "auth.gemini_error", map[string]string{"error": err.Error()})
			return
		}
		runtime.EventsEmit(a.ctx, "auth.gemini_ready", map[string]string{"email": a.auth.Email()})
	}()
	return nil
}

// SignOutGemini removes stored Google tokens.
func (a *App) SignOutGemini() error {
	if err := a.auth.SignOut(); err != nil {
		return err
	}
	runtime.EventsEmit(a.ctx, "auth.gemini_signed_out", nil)
	return nil
}

// JoinMeeting creates an AgentCall call and opens the WebSocket connection.
func (a *App) JoinMeeting(meetURL, botName, triggerWords, botContext, voice string) error {
	a.mu.Lock()
	if a.ws != nil {
		a.mu.Unlock()
		return fmt.Errorf("already in a call")
	}
	a.mu.Unlock()

	cfg, _ := config.Load()

	client := bridge.NewClient(cfg.APIKey, "")
	a.mu.Lock()
	a.client = client
	a.mu.Unlock()

	words := parseTriggerWords(triggerWords)

	req := bridge.CreateCallRequest{
		MeetURL:       meetURL,
		BotName:       botName,
		Mode:          "audio",
		VoiceStrategy: "collaborative",
		Transcription: true,
		Collaborative: &bridge.CollabConfig{
			TriggerWords:      words,
			Context:           botContext,
			BargeInPrevention: true,
		},
	}

	resp, err := client.CreateCall(req)
	if err != nil {
		return fmt.Errorf("could not start call: %w", err)
	}

	wsURL := client.WSURLForCall(resp.CallID)
	ws, err := bridge.Connect(wsURL)
	if err != nil {
		_ = client.DeleteCall(resp.CallID)
		return fmt.Errorf("could not connect to meeting: %w", err)
	}

	a.mu.Lock()
	a.ws = ws
	a.callID = resp.CallID
	a.mu.Unlock()

	var br *brain.Brain
	if a.auth.IsAuthenticated() {
		llmClient := llm.NewClient(botName, botContext)
		br = brain.New(ws, llmClient, a.auth, botName)
		br.Start(a.ctx)
		a.mu.Lock()
		a.br = br
		a.mu.Unlock()
	}

	go a.forwardEvents(ws, br)
	return nil
}

// LeaveCall sends leave and ends the active call.
func (a *App) LeaveCall() error {
	a.mu.Lock()
	ws := a.ws
	callID := a.callID
	client := a.client
	br := a.br
	a.mu.Unlock()

	if ws == nil {
		return nil
	}

	if br != nil {
		br.Stop()
	}
	ws.Close()
	if client != nil && callID != "" {
		_ = client.DeleteCall(callID)
	}

	a.mu.Lock()
	a.ws = nil
	a.callID = ""
	a.br = nil
	a.mu.Unlock()
	return nil
}

// forwardEvents reads from the bridge event channel, emits to the frontend,
// and feeds the brain if active.
func (a *App) forwardEvents(ws *bridge.Bridge, br *brain.Brain) {
	for event := range ws.Events() {
		eventName := event.Normalize()
		if eventName == "" {
			continue
		}
		runtime.EventsEmit(a.ctx, eventName, event)

		if br != nil {
			br.Feed(event)
		}

		if eventName == "call.ended" {
			a.mu.Lock()
			a.ws = nil
			a.callID = ""
			if a.br != nil {
				a.br.Stop()
				a.br = nil
			}
			a.mu.Unlock()
		}
	}
}

func parseTriggerWords(s string) []string {
	var words []string
	for _, w := range strings.Split(s, ",") {
		w = strings.TrimSpace(w)
		if w != "" {
			words = append(words, w)
		}
	}
	return words
}
```

- [ ] **Step 2: Build to verify it compiles**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 3: Run all tests**

```bash
go test ./...
```

Expected: all tests PASS.

- [ ] **Step 4: Commit**

```bash
git add app.go
git commit -m "feat(app): wire Gemini auth + brain into JoinMeeting lifecycle"
```

---

## Task 7: Frontend — auth UI and call indicator

Add the "Sign in with Google" section to the settings screen and the "Gemini active" chip to the call screen.

**Files:**
- Modify: `frontend/index.html`
- Modify: `frontend/src/main.js`

- [ ] **Step 1: Add HTML to `frontend/index.html`**

In `frontend/index.html`, find the settings screen section (`id="screen-setup"`) and add the Gemini auth block. The full updated settings screen div should look like this (replace only the `screen-setup` div):

```html
<div id="screen-setup" class="screen hidden">
  <h1>AgentCall Setup</h1>

  <label for="setup-apikey">AgentCall API Key</label>
  <input id="setup-apikey" type="password" placeholder="ak_ac_…" autocomplete="off" />
  <p id="setup-error" class="error hidden"></p>
  <button id="setup-save">Save &amp; Continue</button>

  <hr class="divider" />

  <div id="gemini-section">
    <h2>AI Brain (Gemini)</h2>
    <p class="hint">Sign in with Google to enable live meeting intelligence powered by Gemini.</p>

    <div id="gemini-signed-out">
      <button id="gemini-signin">Sign in with Google</button>
      <p id="gemini-auth-error" class="error hidden"></p>
    </div>

    <div id="gemini-signed-in" class="hidden">
      <span id="gemini-email" class="gemini-email"></span>
      <button id="gemini-signout" class="secondary">Sign out</button>
    </div>
  </div>
</div>
```

In the call screen div (`id="screen-call"`), find the status line and add the chip right after the status text span:

```html
<span id="gemini-chip" class="gemini-chip hidden">● Gemini active</span>
```

The full status bar line should look like:
```html
<div class="status-bar">
  <span id="call-status-dot" class="status-dot joining"></span>
  <span id="call-status-text">Joining meeting...</span>
  <span id="gemini-chip" class="gemini-chip hidden">● Gemini active</span>
</div>
```

Add these CSS rules to `frontend/src/style.css`:

```css
hr.divider {
  border: none;
  border-top: 1px solid #333;
  margin: 1.5rem 0;
}

#gemini-section h2 {
  font-size: 1rem;
  margin-bottom: 0.25rem;
}

.hint {
  color: #888;
  font-size: 0.85rem;
  margin-bottom: 0.75rem;
}

.gemini-email {
  color: #aaa;
  font-size: 0.9rem;
  margin-right: 0.75rem;
}

button.secondary {
  background: transparent;
  border: 1px solid #555;
  color: #ccc;
  padding: 0.3rem 0.75rem;
  font-size: 0.85rem;
  cursor: pointer;
  border-radius: 4px;
}

button.secondary:hover {
  border-color: #aaa;
  color: #fff;
}

.gemini-chip {
  margin-left: auto;
  font-size: 0.75rem;
  color: #4caf50;
}
```

- [ ] **Step 2: Add JS to `frontend/src/main.js`**

Append the following to the end of `frontend/src/main.js`:

```js
// ── Gemini auth ───────────────────────────────────────────────────────────

async function refreshGeminiUI() {
  const status = await window.go.main.App.GetGeminiStatus();
  if (status.authenticated) {
    document.getElementById('gemini-signed-out').classList.add('hidden');
    document.getElementById('gemini-signed-in').classList.remove('hidden');
    document.getElementById('gemini-email').textContent = status.email;
  } else {
    document.getElementById('gemini-signed-out').classList.remove('hidden');
    document.getElementById('gemini-signed-in').classList.add('hidden');
    document.getElementById('gemini-email').textContent = '';
  }
}

// Populate auth state when settings screen opens.
document.getElementById('join-settings').addEventListener('click', async () => {
  await refreshGeminiUI();
  show('screen-setup');
});

// Also refresh on load if already on setup screen.
window.addEventListener('load', async () => {
  await refreshGeminiUI();
});

document.getElementById('gemini-signin').addEventListener('click', async () => {
  hideError('gemini-auth-error');
  document.getElementById('gemini-signin').disabled = true;
  document.getElementById('gemini-signin').textContent = 'Opening browser…';
  try {
    await window.go.main.App.StartGeminiAuth();
    // UI update happens via the auth.gemini_ready Wails event below.
  } catch (e) {
    showError('gemini-auth-error', 'Could not start sign-in: ' + e);
    document.getElementById('gemini-signin').disabled = false;
    document.getElementById('gemini-signin').textContent = 'Sign in with Google';
  }
});

document.getElementById('gemini-signout').addEventListener('click', async () => {
  await window.go.main.App.SignOutGemini();
});

// Wails events from the backend OAuth flow.
window.runtime.EventsOn('auth.gemini_ready', async (ev) => {
  document.getElementById('gemini-signin').disabled = false;
  document.getElementById('gemini-signin').textContent = 'Sign in with Google';
  await refreshGeminiUI();
});

window.runtime.EventsOn('auth.gemini_error', (ev) => {
  document.getElementById('gemini-signin').disabled = false;
  document.getElementById('gemini-signin').textContent = 'Sign in with Google';
  showError('gemini-auth-error', ev.error || 'Sign-in failed.');
});

window.runtime.EventsOn('auth.gemini_signed_out', async () => {
  await refreshGeminiUI();
});

// ── Gemini active chip ────────────────────────────────────────────────────

async function updateGeminiChip() {
  const status = await window.go.main.App.GetGeminiStatus();
  const chip = document.getElementById('gemini-chip');
  if (status.authenticated) {
    chip.classList.remove('hidden');
  } else {
    chip.classList.add('hidden');
  }
}

// Update chip when the bot has successfully joined — Gemini auth is known by then.
window.runtime.EventsOn('call.bot_ready', async () => {
  await updateGeminiChip();
});
```

- [ ] **Step 3: Build and launch the app**

```bash
wails dev
```

Expected: app launches. Settings screen shows the Gemini section. Clicking "Sign in with Google" opens the browser.

> **Manual test checklist:**
> - [ ] Settings screen shows "Sign in with Google" button when not signed in
> - [ ] Clicking the button opens the system browser to Google OAuth
> - [ ] After approving, the UI updates to show the email address
> - [ ] "Sign out" returns to the sign-in button
> - [ ] Joining a meeting when authenticated shows "Gemini active" chip
> - [ ] Joining when not authenticated shows no chip — meeting still works as before

- [ ] **Step 4: Commit**

```bash
git add frontend/index.html frontend/src/main.js frontend/src/style.css
git commit -m "feat(frontend): Gemini sign-in UI and call screen active indicator"
```

---

## Final verification

- [ ] **Run all tests one last time**

```bash
go test ./... -v
```

Expected: all packages pass.

- [ ] **Build release binary**

```bash
wails build
```

Expected: binary produced in `build/bin/` with no errors.
