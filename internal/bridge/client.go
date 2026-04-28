package bridge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultAPIBase = "https://api.agentcall.dev"

type Client struct {
	apiKey  string
	apiBase string
	http    *http.Client
}

func NewClient(apiKey, apiBase string) *Client {
	if apiBase == "" {
		apiBase = defaultAPIBase
	}
	return &Client{
		apiKey:  apiKey,
		apiBase: apiBase,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) CreateCall(req CreateCallRequest) (*CreateCallResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	r, err := http.NewRequest(http.MethodPost, c.apiBase+"/v1/calls", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	r.Header.Set("Authorization", "Bearer "+c.apiKey)
	r.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(r)
	if err != nil {
		return nil, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create call failed (%d): %s", resp.StatusCode, string(b))
	}

	var out CreateCallResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &out, nil
}

func (c *Client) DeleteCall(callID string) error {
	r, err := http.NewRequest(http.MethodDelete, c.apiBase+"/v1/calls/"+callID, nil)
	if err != nil {
		return err
	}
	r.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(r)
	if err != nil {
		return fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete call failed (%d): %s", resp.StatusCode, string(b))
	}
	return nil
}

// WSURLForCall builds the WebSocket URL for a call ID.
// The WS URL uses the api_key as a query parameter (matching bridge.py behaviour).
func (c *Client) WSURLForCall(callID string) string {
	wsBase := replaceScheme(c.apiBase)
	return fmt.Sprintf("%s/v1/calls/%s/ws?api_key=%s", wsBase, callID, c.apiKey)
}

func replaceScheme(url string) string {
	switch {
	case len(url) >= 8 && url[:8] == "https://":
		return "wss://" + url[8:]
	case len(url) >= 7 && url[:7] == "http://":
		return "ws://" + url[7:]
	}
	return url
}
