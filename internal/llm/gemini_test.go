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
	// 12 pairs = 24 entries, below the 40-entry cap, so all kept + 1 new = 25
	if len(req.Contents) != 25 {
		t.Errorf("expected 25 contents, got %d", len(req.Contents))
	}
}

func TestHistoryCapAt40Entries(t *testing.T) {
	c := llm.NewClient("Juno", "")
	for i := 0; i < 21; i++ { // 21 pairs = 42 entries, exceeds cap of 40
		c.RecordTurn("question", "answer")
	}
	req := c.BuildRequest("new question")
	// capped to 40 entries + 1 new = 41
	if len(req.Contents) != 41 {
		t.Errorf("expected 41 contents, got %d", len(req.Contents))
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
