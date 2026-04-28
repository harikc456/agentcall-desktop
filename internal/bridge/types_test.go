package bridge_test

import (
	"encoding/json"
	"testing"

	"agentcall-desktop/internal/bridge"
)

func TestCreateCallRequestJSON(t *testing.T) {
	req := bridge.CreateCallRequest{
		MeetURL:       "https://meet.google.com/abc",
		BotName:       "Juno",
		Mode:          "audio",
		VoiceStrategy: "collaborative",
		Transcription: true,
		Collaborative: &bridge.CollabConfig{
			TriggerWords: []string{"juno", "june"},
			Context:      "You are a meeting assistant.",
		},
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var m map[string]interface{}
	json.Unmarshal(data, &m)
	if m["meet_url"] != "https://meet.google.com/abc" {
		t.Errorf("meet_url wrong: %v", m["meet_url"])
	}
	if m["voice_strategy"] != "collaborative" {
		t.Errorf("voice_strategy wrong: %v", m["voice_strategy"])
	}
	collab, ok := m["collaborative"].(map[string]interface{})
	if !ok {
		t.Fatal("collaborative field missing or wrong type")
	}
	words, _ := collab["trigger_words"].([]interface{})
	if len(words) != 2 {
		t.Errorf("trigger_words len: got %d want 2", len(words))
	}
}

func TestEventNormalize(t *testing.T) {
	// Events use either "event" or "type" field — Normalize returns whichever is set.
	e := bridge.Event{Type: "transcript.final", Text: "Hello"}
	if e.Normalize() != "transcript.final" {
		t.Errorf("Normalize: got %q want %q", e.Normalize(), "transcript.final")
	}
	e2 := bridge.Event{EventType: "call.bot_ready"}
	if e2.Normalize() != "call.bot_ready" {
		t.Errorf("Normalize: got %q want %q", e2.Normalize(), "call.bot_ready")
	}
}
