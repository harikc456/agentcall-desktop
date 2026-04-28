package main

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"agentcall-desktop/internal/bridge"
	"agentcall-desktop/internal/config"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails application struct. All exported methods are bound to the frontend.
type App struct {
	ctx    context.Context
	mu     sync.Mutex
	client *bridge.Client
	ws     *bridge.Bridge
	callID string
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// beforeClose is called when the user closes the window.
// Returns false to allow close (true would block it).
func (a *App) beforeClose(ctx context.Context) bool {
	a.cleanup()
	return false
}

func (a *App) cleanup() {
	a.mu.Lock()
	ws := a.ws
	callID := a.callID
	client := a.client
	a.ws = nil
	a.callID = ""
	a.mu.Unlock()

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

// JoinMeeting creates an AgentCall call and opens the WebSocket connection.
func (a *App) JoinMeeting(meetURL, botName, triggerWords, botContext, voice string) error {
	a.mu.Lock()
	if a.ws != nil {
		a.mu.Unlock()
		return fmt.Errorf("already in a call")
	}
	a.mu.Unlock()

	cfg, _ := config.Load()

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

	go a.forwardEvents(ws)
	return nil
}

// LeaveCall sends leave and ends the active call.
func (a *App) LeaveCall() error {
	a.mu.Lock()
	ws := a.ws
	callID := a.callID
	client := a.client
	a.mu.Unlock()

	if ws == nil {
		return nil
	}

	ws.Close()
	if client != nil && callID != "" {
		_ = client.DeleteCall(callID)
	}

	a.mu.Lock()
	a.ws = nil
	a.callID = ""
	a.mu.Unlock()
	return nil
}

// forwardEvents reads from the bridge event channel and emits to the frontend.
func (a *App) forwardEvents(ws *bridge.Bridge) {
	for event := range ws.Events() {
		eventName := event.Normalize()
		if eventName == "" {
			continue
		}
		runtime.EventsEmit(a.ctx, eventName, event)

		if eventName == "call.ended" {
			a.mu.Lock()
			a.ws = nil
			a.callID = ""
			a.mu.Unlock()
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
