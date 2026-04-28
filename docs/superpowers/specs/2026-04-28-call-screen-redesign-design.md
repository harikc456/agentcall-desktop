# Call Screen Redesign

**Date:** 2026-04-28  
**Status:** Approved

## Goal

Replace the current flat call screen (status dot + stacked sections) with a chat-bubble layout that clearly communicates bot state and gives the transcript visual priority.

## Layout

Single column, 420px max-width, four vertical zones top to bottom:

1. **Status bar** — slim pill, full width
2. **Conversation feed** — flex-grow, scrollable chat bubbles
3. **Credits warning** — shown only when credits are low (unchanged behaviour)
4. **Bottom row** — participant pills (left) + Leave button (right)

## Status Bar

The bar background and indicator change per state:

| State | Indicator | Text | Bar background |
|---|---|---|---|
| `joining` | Amber static dot | "Joining meeting…" | `#1a1a1a` |
| `ready` / listening | Green pulsing dot | "{botName} is listening" | `#1a1a1a` |
| `responding` | 5 green waveform bars (animated) | "{botName} is responding…" | `#1a2e1a` with subtle green border |
| `alone` | Amber static dot | "Alone in meeting — waiting for participants" | `#1a1a1a` |

Gemini chip stays top-right of the bar (hidden when Gemini inactive, unchanged).

**Responding state** is triggered on every `voice.text` event and reverts to `ready` (listening) after a 2-second debounce with no new `voice.text` events.

## Conversation Feed

Replaces the separate PARTICIPANTS and LIVE TRANSCRIPT sections.

- Bot messages: left-aligned, dark green bubble (`#1e3a2f`), speaker label "Juno" in green above
- Human messages: right-aligned, dark bubble (`#1e1e1e`), speaker name in muted grey above
- Auto-scrolls to bottom on new messages
- Empty state: centred hint text "Conversation will appear here…"

## Bottom Row

Replaces the standalone Leave button and participants list.

- **Left**: participant pills (rounded tags, `👤 Name`). If no participants, pills are absent — no placeholder text needed since the status bar already communicates the `alone` state.
- **Right**: Leave button (danger red, unchanged style, compact `padding: 8px 14px`)

## Bot State Machine

```
joining → ready (on call.bot_ready)
ready   → responding (on voice.text)
responding → ready (2s debounce after last voice.text)
ready   → alone (on participant.left when participants = 0)
alone   → ready (on participant.joined)
```

## Files Changed

- `frontend/index.html` — restructure screen-call markup
- `frontend/style.css` — new classes: `.status-bar`, `.waveform`, `.chat-feed`, `.bubble`, `.bubble-bot`, `.bubble-human`, `.bottom-row`, `.participant-pill`; update `.status-dot` animation
- `frontend/src/main.js` — new `setBotState()` function, responding debounce timer, updated event handlers, updated `resetCallScreen()`

## Out of Scope

- No changes to join screen, setup screen, or backend
- No sound or haptic feedback
- No message timestamps
