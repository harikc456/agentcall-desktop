package bridge_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"agentcall-desktop/internal/bridge"
)

func TestCreateCall(t *testing.T) {
	var captured map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/calls" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("auth header: %q", r.Header.Get("Authorization"))
		}
		json.NewDecoder(r.Body).Decode(&captured)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{
			"call_id": "call-123",
			"ws_url":  "wss://api.agentcall.dev/v1/calls/call-123/ws",
			"status":  "bot_joining",
		})
	}))
	defer srv.Close()

	c := bridge.NewClient("test-key", srv.URL)
	resp, err := c.CreateCall(bridge.CreateCallRequest{
		MeetURL:       "https://meet.google.com/abc",
		BotName:       "Juno",
		Mode:          "audio",
		VoiceStrategy: "collaborative",
		Transcription: true,
	})
	if err != nil {
		t.Fatalf("CreateCall error: %v", err)
	}
	if resp.CallID != "call-123" {
		t.Errorf("CallID: got %q want %q", resp.CallID, "call-123")
	}
	if captured["meet_url"] != "https://meet.google.com/abc" {
		t.Errorf("meet_url in body: %v", captured["meet_url"])
	}
}

func TestDeleteCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/v1/calls/call-123" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ended"})
	}))
	defer srv.Close()

	c := bridge.NewClient("test-key", srv.URL)
	if err := c.DeleteCall("call-123"); err != nil {
		t.Fatalf("DeleteCall error: %v", err)
	}
}

func TestCreateCallHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid api key"}`))
	}))
	defer srv.Close()

	c := bridge.NewClient("bad-key", srv.URL)
	_, err := c.CreateCall(bridge.CreateCallRequest{MeetURL: "x", BotName: "y"})
	if err == nil {
		t.Fatal("expected error for 401")
	}
}
