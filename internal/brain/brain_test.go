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
