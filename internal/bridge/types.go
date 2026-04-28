package bridge

// CreateCallRequest is sent to POST /v1/calls.
type CreateCallRequest struct {
	MeetURL       string        `json:"meet_url"`
	BotName       string        `json:"bot_name"`
	Mode          string        `json:"mode"`
	VoiceStrategy string        `json:"voice_strategy"`
	Transcription bool          `json:"transcription"`
	Collaborative *CollabConfig `json:"collaborative,omitempty"`
}

// CollabConfig holds collaborative voice intelligence settings.
type CollabConfig struct {
	TriggerWords      []string `json:"trigger_words"`
	Context           string   `json:"context"`
	BargeInPrevention bool     `json:"barge_in_prevention"`
}

// CreateCallResponse is returned by POST /v1/calls (HTTP 201).
type CreateCallResponse struct {
	CallID string `json:"call_id"`
	WSURL  string `json:"ws_url"`
	Status string `json:"status"`
}

// Event is a single WebSocket event from the AgentCall server.
// Lifecycle events use "event"; transcript/meeting events use "type".
type Event struct {
	EventType   string       `json:"event,omitempty"`
	Type        string       `json:"type,omitempty"`
	CallID      string       `json:"call_id,omitempty"`
	Text        string       `json:"text,omitempty"`
	Speaker     *Speaker     `json:"speaker,omitempty"`
	Participant *Participant `json:"participant,omitempty"`
	Name        string       `json:"name,omitempty"`
	Reason      string       `json:"reason,omitempty"`
	Sender      string       `json:"sender,omitempty"`
	Message     string       `json:"message,omitempty"`
}

// Normalize returns whichever of EventType or Type is non-empty.
func (e Event) Normalize() string {
	if e.EventType != "" {
		return e.EventType
	}
	return e.Type
}

type Speaker struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Participant struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Command is a JSON command sent to the WebSocket server.
type Command struct {
	Command string `json:"command"`
	Text    string `json:"text,omitempty"`
	Voice   string `json:"voice,omitempty"`
	Message string `json:"message,omitempty"`
}

// ContextUpdateCmd updates the AI's context mid-call.
type ContextUpdateCmd struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// TriggerSpeakCmd triggers the AI to speak.
type TriggerSpeakCmd struct {
	Type    string `json:"type"`
	Text    string `json:"text"`
	Speaker string `json:"speaker,omitempty"`
}
