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
	go srv.Serve(ln) //nolint:errcheck
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
	json.NewDecoder(resp.Body).Decode(&info) //nolint:errcheck
	return info.Email, nil
}
