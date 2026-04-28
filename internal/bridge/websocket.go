package bridge

import (
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"
)

// Bridge holds an active WebSocket connection to an AgentCall call.
type Bridge struct {
	conn   *websocket.Conn
	events chan Event
	done   chan struct{}
	mu     sync.Mutex
}

// Connect opens a WebSocket to wsURL and starts the read loop.
// Events are available via Events(). The caller must eventually call Close().
func Connect(wsURL string) (*Bridge, error) {
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return nil, err
	}
	b := &Bridge{
		conn:   conn,
		events: make(chan Event, 64),
		done:   make(chan struct{}),
	}
	go b.readLoop()
	return b, nil
}

// Events returns the channel of incoming events. Closed when the WS closes.
func (b *Bridge) Events() <-chan Event {
	return b.events
}

// SendCommand writes a command to the WebSocket as JSON.
func (b *Bridge) SendCommand(cmd Command) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.conn.WriteJSON(cmd)
}

// Close sends a leave command, then closes the WebSocket.
func (b *Bridge) Close() {
	// Best-effort leave — ignore error (call may already be ended).
	_ = b.SendCommand(Command{Command: "leave"})
	b.conn.Close()
}

func (b *Bridge) readLoop() {
	defer close(b.events)
	for {
		_, msg, err := b.conn.ReadMessage()
		if err != nil {
			return
		}
		var event Event
		if err := json.Unmarshal(msg, &event); err != nil {
			continue
		}
		select {
		case b.events <- event:
		case <-b.done:
			return
		}
	}
}
