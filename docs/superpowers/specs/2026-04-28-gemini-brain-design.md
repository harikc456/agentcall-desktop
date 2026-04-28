# Gemini Brain — Design Spec

**Date:** 2026-04-28  
**Status:** Approved  

## Problem

The agentcall-desktop app joins meetings in collaborative mode, but has no active intelligence — GetSun speaks only from a static context string the user pre-fills before the call. The `join-meeting` skill in Claude Code provides the "brain" (LLM loop), but this requires Claude Code running alongside the app. The desktop app should contain its own LLM brain so it works standalone.

## Goal

Add a Gemini-powered LLM loop inside the desktop app. When the user is signed in with Google, each `transcript.final` event from the meeting is sent to Gemini, and the response is fed back to GetSun via `voice.context_update` + `trigger.speak`. GetSun speaks the LLM-generated response. If the user has not signed in, the app works exactly as today.

## Architecture

Three new Go packages added under `internal/`:

```
internal/
  auth/    — Google OAuth PKCE flow, token storage and refresh
  llm/     — Gemini REST API client, conversation history
  brain/   — LLM loop goroutine, wires auth/llm into the call lifecycle
```

No changes to `internal/bridge`, `internal/config`, or `go.mod` websocket handling. The brain sits above existing plumbing and reuses the WebSocket send path already present in `app.go`.

## Section 1: OAuth Flow

### Client registration

The app ships with a hardcoded Google OAuth `client_id` (registered once by the developer in Google Cloud Console as an "installed application"). No `client_secret` is required — PKCE eliminates the need for it in desktop apps.

### Sign-in flow

1. Generate random PKCE code verifier (32 bytes, base64url-encoded) and SHA-256 challenge.
2. Pick a random free port in range 49152–65535; start a local HTTP server at `http://localhost:{port}/callback`.
3. Open the system browser to Google's auth URL with scopes:
   - `https://www.googleapis.com/auth/generative-language`
   - `email`
4. User approves in browser → redirects to `localhost:{port}/callback?code=...`.
5. Exchange code + verifier for access token + refresh token.
6. Shut down local HTTP server.
7. Emit Wails event `auth.gemini_ready` to frontend with `{email}`.

### Token storage

File: `~/.agentcall/tokens.json`, mode `0600`.

```json
{
  "gemini": {
    "access_token": "...",
    "refresh_token": "...",
    "expiry": "2026-04-28T15:00:00Z",
    "email": "user@gmail.com"
  }
}
```

Access token is refreshed automatically before any Gemini API call if within 60 seconds of expiry. Sign-out deletes the `gemini` key and rewrites the file.

### New Wails-bound methods on `App`

| Method | Signature | Description |
|--------|-----------|-------------|
| `StartGeminiAuth` | `() error` | Starts OAuth PKCE flow, opens browser |
| `GetGeminiStatus` | `() GeminiStatus` | Returns `{Authenticated bool, Email string}` |
| `SignOutGemini` | `() error` | Clears tokens, emits `auth.gemini_signed_out` |

## Section 2: Gemini Client

**Model:** `gemini-2.0-flash`  
**Endpoint:** `https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent`  
**Auth:** `Authorization: Bearer {access_token}` header  

### Conversation history

`llm.Client` holds a sliding window of the last 20 turns (user + model alternating). Each `Chat()` call appends the new transcript as a user turn and the LLM response as a model turn.

### System prompt

Built at call-start from the bot's name and the user's `context` field:

```
You are {botName}, an AI meeting assistant. Respond conversationally and concisely (2-3 sentences max).
{context field if non-empty}
```

### `Chat()` method

```go
func (c *Client) Chat(ctx context.Context, transcript string) (string, error)
```

Sends system prompt + full conversation history + new transcript. Returns the LLM text response. On API error, returns the error — the brain logs it and skips the turn without crashing the call.

## Section 3: Brain Loop

### Lifecycle

`brain.Brain` is created in `App.JoinMeeting` when `GetGeminiStatus().Authenticated` is true. It is stopped (via context cancellation) in `App.LeaveCall` and `App.cleanup`.

### Event channel

`forwardEvents` is extended to also send events to a `chan bridge.Event` consumed by the brain goroutine. The brain only subscribes to `transcript.final` and `call.ended`.

### Per-transcript logic

For each `transcript.final` event:

1. **Skip bot turns** — if `event.Speaker.Name == botName`, ignore (prevents feedback loop).
2. **Call Gemini** — `llm.Client.Chat(ctx, event.Text)`.
3. **On error** — log, continue. The call is unaffected.
4. **Send context update** — WebSocket command:
   ```json
   {"type": "voice.context_update", "text": "<LLM response>"}
   ```
5. **Trigger speak** — WebSocket command:
   ```json
   {"type": "trigger.speak", "text": "<original transcript>", "speaker": "<speaker name>"}
   ```
   GetSun reads from the updated context and speaks naturally.

On `call.ended` → goroutine returns.

### Without Gemini

If the user is not signed in, `App.JoinMeeting` does not start the brain. The call proceeds exactly as today: GetSun uses the static context from the `context` field.

## Frontend Changes

**Settings screen:**
- Add "Sign in with Google" button when not authenticated.
- When authenticated: show `{email}` and a "Sign out" link.
- No other fields added — provider selection is deferred until more providers are added.
- Listen to Wails events `auth.gemini_ready` (payload: `{email}`) and `auth.gemini_signed_out` to update the UI in real-time without requiring a screen reload.

**Call screen:**
- Add small status chip: "Gemini active" (green dot) when brain is running, hidden otherwise.

## Out of Scope

- OpenAI and Claude provider support (deferred — no OAuth available from those providers today).
- Configurable Gemini model selection.
- Brain behavior customization beyond the existing `context` and `botName` fields.
- Streaming Gemini responses (full response before speaking is acceptable for meeting cadence).
