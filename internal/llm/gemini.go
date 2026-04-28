package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	geminiEndpoint  = "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent"
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
	botName    string
	sysPrompt  string
	history    []Content
	mu         sync.Mutex
	httpClient *http.Client
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
	return &Client{
		botName:    botName,
		sysPrompt:  prompt,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// BuildRequest constructs a GenerateRequest from history + the new transcript.
// Exposed for testing; callers normally use Chat().
func (c *Client) BuildRequest(transcript string) GenerateRequest {
	c.mu.Lock()
	defer c.mu.Unlock()

	history := c.history
	if len(history) > maxHistoryTurns*2 {
		history = history[len(history)-maxHistoryTurns*2:]
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
// apiKey must be a Gemini API key from https://aistudio.google.com/app/apikey
func (c *Client) Chat(ctx context.Context, apiKey, transcript string) (string, error) {
	req := c.BuildRequest(transcript)

	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, geminiEndpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("x-goog-api-key", apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("gemini request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		if len(respBody) > 512 {
			respBody = respBody[:512]
		}
		return "", fmt.Errorf("gemini %d: %s", resp.StatusCode, respBody)
	}

	text, err := ParseResponse(respBody)
	if err != nil {
		return "", err
	}

	c.RecordTurn(transcript, text)
	return text, nil
}
