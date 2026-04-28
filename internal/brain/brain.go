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
