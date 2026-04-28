package main

import (
	"context"
	"fmt"
	"strings"
	"sync"

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
}

// GeminiStatus is returned to the frontend.
type GeminiStatus struct {
	Enabled bool `json:"enabled"`
}

func NewApp() *App {
	return &App{}
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

// GetGeminiStatus returns whether a Gemini API key is configured.
func (a *App) GetGeminiStatus() GeminiStatus {
	cfg, err := config.Load()
	if err != nil {
		return GeminiStatus{}
	}
	return GeminiStatus{Enabled: cfg.GeminiAPIKey != ""}
}

// JoinMeeting creates an AgentCall call and opens the WebSocket connection.
// If a Gemini API key is configured, the brain is started automatically.
func (a *App) JoinMeeting(meetURL, botName, triggerWords, botContext, voice string) error {
	a.mu.Lock()
	if a.ws != nil {
		a.mu.Unlock()
		return fmt.Errorf("already in a call")
	}
	a.mu.Unlock()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("could not load config: %w", err)
	}
	if cfg.APIKey == "" {
		return fmt.Errorf("API key not configured — open Settings and enter your AgentCall API key")
	}

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
	if cfg.GeminiAPIKey != "" {
		llmClient := llm.NewClient(botName, botContext)
		br = brain.New(ws, llmClient, cfg.GeminiAPIKey, botName)
		br.Start(a.ctx)
		a.mu.Lock()
		a.br = br
		a.mu.Unlock()
	}

	go a.forwardEvents(ws, br)
	return nil
}

// LeaveCall ends the active call and stops the brain if running.
func (a *App) LeaveCall() error {
	a.cleanup()
	return nil
}

// forwardEvents reads WebSocket events, emits to the frontend, and feeds the brain if active.
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
			br := a.br
			a.br = nil
			a.mu.Unlock()
			if br != nil {
				br.Stop()
			}
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
