# Call Screen Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the flat call screen with a chat-bubble layout that shows bot state (listening / responding / alone) clearly and renders the transcript as a conversation feed.

**Architecture:** Three files change — `index.html` (markup restructure), `style.css` (new call-screen classes, remove old ones), `main.js` (new `setBotState()`, `appendMessage()`, debounce timer). No backend changes.

**Tech Stack:** Vanilla JS, HTML, CSS; Wails v2 (Go desktop shell). Build command: `/home/harikrishnan-c/go/bin/wails build -tags webkit2_41`

---

### Task 1: Restructure screen-call HTML

**Files:**
- Modify: `frontend/index.html`

Replace the entire `screen-call` div content. The old markup has `.call-header`, `.section-label`, `.participants-list`, `.transcript-feed`, and a standalone `#call-leave`. The new markup has four zones: status bar, chat feed, warning, bottom row.

- [ ] **Step 1: Replace the screen-call block**

Open `frontend/index.html`. Find the `<!-- Screen: Active Call -->` comment (line ~77). Replace the entire `<div id="screen-call" ...>` block with:

```html
<!-- Screen: Active Call -->
<div id="screen-call" class="screen hidden">
  <div id="call-status-bar" class="status-bar">
    <div id="call-status-indicator" class="status-indicator">
      <div id="call-status-dot" class="status-dot joining"></div>
    </div>
    <span id="call-status-text">Joining meeting...</span>
    <span id="gemini-chip" class="gemini-chip hidden">● Gemini</span>
  </div>
  <div id="call-feed" class="chat-feed">
    <span class="empty-hint">Conversation will appear here…</span>
  </div>
  <div id="call-warning" class="warning hidden"></div>
  <div class="bottom-row">
    <div id="call-participants" class="participants-strip"></div>
    <button id="call-leave" class="btn-leave">Leave</button>
  </div>
</div>
```

- [ ] **Step 2: Verify HTML structure is valid**

```bash
grep -n "screen-call\|call-status-bar\|call-feed\|call-participants\|call-leave\|bottom-row" frontend/index.html
```

Expected: each id appears exactly once, `bottom-row` contains `call-participants` and `call-leave`.

- [ ] **Step 3: Commit**

```bash
git add frontend/index.html
git commit -m "refactor(html): restructure call screen into status-bar / chat-feed / bottom-row layout"
```

---

### Task 2: Replace old call-screen CSS with new classes

**Files:**
- Modify: `frontend/style.css`

Remove the old call-screen block (`.call-header`, `.status-dot`, `.section-label`, `.participants-list`, `.participant`, `.transcript-feed`, `.transcript-line`). Add new classes for the status bar, waveform animation, chat feed, bubbles, bottom row, and participant pills.

- [ ] **Step 1: Remove old call-screen styles**

In `frontend/style.css`, delete everything from the `/* Active call screen */` comment (line ~153) down to and including `.transcript-line.bot .text { color: #bfdbfe; }` (line ~212). Do **not** remove the Gemini section below it.

- [ ] **Step 2: Add new call-screen styles**

In place of the removed block, insert:

```css
/* ── Call screen ─────────────────────────────────────────────────────── */
.status-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  background: #1a1a1a;
  border-radius: 10px;
  border: 1px solid transparent;
  margin-bottom: 12px;
  transition: background .3s, border-color .3s;
}
.status-bar.responding {
  background: #1a2e1a;
  border-color: #14532d66;
}

.status-indicator {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  flex-shrink: 0;
}

.status-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}
.status-dot.joining { background: #f59e0b; }
.status-dot.ready   { background: #22c55e; animation: dot-pulse 2s ease-in-out infinite; }
.status-dot.alone   { background: #f59e0b; }

@keyframes dot-pulse {
  0%, 100% { box-shadow: 0 0 4px #22c55e66; }
  50%       { box-shadow: 0 0 12px #22c55ecc; }
}

.waveform {
  display: flex;
  gap: 2px;
  align-items: center;
  height: 16px;
}
.waveform span {
  display: block;
  width: 3px;
  height: 4px;
  background: #22c55e;
  border-radius: 2px;
  animation: waveform-bar .7s ease-in-out infinite;
}
.waveform span:nth-child(1) { animation-delay: 0.00s; }
.waveform span:nth-child(2) { animation-delay: 0.10s; }
.waveform span:nth-child(3) { animation-delay: 0.20s; }
.waveform span:nth-child(4) { animation-delay: 0.30s; }
.waveform span:nth-child(5) { animation-delay: 0.40s; }

@keyframes waveform-bar {
  0%, 100% { height: 4px; }
  50%       { height: 14px; }
}

#call-status-text {
  flex: 1;
  font-size: 12px;
  font-weight: 600;
}

.chat-feed {
  flex: 1;
  background: #161616;
  border-radius: 10px;
  padding: 12px;
  height: 300px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-bottom: 8px;
}

.empty-hint { color: #444; font-size: 12px; text-align: center; margin: auto; }

.bubble-wrapper {
  display: flex;
  flex-direction: column;
  max-width: 82%;
}
.bubble-wrapper.bubble-bot  { align-self: flex-start; }
.bubble-wrapper.bubble-human { align-self: flex-end; }

.bubble-label {
  font-size: 10px;
  margin-bottom: 3px;
  padding: 0 4px;
}
.bubble-bot  .bubble-label { color: #22c55e; }
.bubble-human .bubble-label { color: #6b7280; text-align: right; }

.bubble {
  padding: 8px 12px;
  font-size: 12px;
  line-height: 1.5;
  word-break: break-word;
}
.bubble-bot  .bubble {
  background: #1e3a2f;
  border-radius: 2px 10px 10px 10px;
  color: #d1fae5;
}
.bubble-human .bubble {
  background: #1e1e1e;
  border-radius: 10px 2px 10px 10px;
  color: #d1d5db;
}

.bottom-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 4px;
}

.participants-strip {
  flex: 1;
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
}

.participant-pill {
  background: #1e1e1e;
  border-radius: 20px;
  padding: 4px 10px;
  font-size: 11px;
  color: #9ca3af;
}

.btn-leave {
  background: #7f1d1d;
  border: none;
  border-radius: 8px;
  color: #fca5a5;
  font-size: 12px;
  font-weight: 700;
  padding: 8px 16px;
  cursor: pointer;
  white-space: nowrap;
  transition: background .15s;
  flex-shrink: 0;
}
.btn-leave:hover { background: #991b1b; }
.btn-leave:disabled { opacity: .5; cursor: not-allowed; }
```

- [ ] **Step 3: Commit**

```bash
git add frontend/style.css
git commit -m "refactor(css): replace old call-screen styles with status-bar, chat-feed, bubble, bottom-row classes"
```

---

### Task 3: Add `setBotState()` and the responding debounce

**Files:**
- Modify: `frontend/src/main.js`

Replace the old `setStatus()` function with `setBotState()`, which updates the status bar class, swaps the indicator between dot and waveform, and sets the text. Add a `respondingTimer` module-level variable for the debounce.

- [ ] **Step 1: Add respondingTimer variable**

At the top of `main.js`, after `let botName = 'Juno';`, add:

```js
let respondingTimer = null;
```

- [ ] **Step 2: Replace setStatus() with setBotState()**

Find and delete the old `setStatus(state)` function (the one that sets `dot.className` and `text.textContent`). Replace it with:

```js
function setBotState(state) {
  const bar       = document.getElementById('call-status-bar');
  const indicator = document.getElementById('call-status-indicator');
  const text      = document.getElementById('call-status-text');

  bar.className = 'status-bar ' + state;

  if (state === 'responding') {
    indicator.innerHTML =
      '<div class="waveform">' +
      '<span></span><span></span><span></span><span></span><span></span>' +
      '</div>';
  } else {
    indicator.innerHTML = '<div class="status-dot ' + state + '"></div>';
  }

  const labels = {
    joining:    'Joining meeting...',
    ready:      botName + ' is listening',
    responding: botName + ' is responding…',
    alone:      'Alone in meeting — waiting for participants',
  };
  text.textContent = labels[state] || state;
}
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/main.js
git commit -m "feat(js): add setBotState() with waveform indicator and state-aware status bar"
```

---

### Task 4: Replace appendTranscript() with appendMessage()

**Files:**
- Modify: `frontend/src/main.js`

The old `appendTranscript(speaker, text, isBot)` writes to `#call-transcript` using `innerHTML`. Replace it with `appendMessage(speaker, text, isBot)` that writes to `#call-feed` using safe DOM methods.

- [ ] **Step 1: Replace appendTranscript() with appendMessage()**

Find and delete the old `appendTranscript(speaker, text, isBot)` function. Replace it with:

```js
function appendMessage(speaker, text, isBot) {
  const feed = document.getElementById('call-feed');
  const hint = feed.querySelector('.empty-hint');
  if (hint) hint.remove();

  const wrapper = document.createElement('div');
  wrapper.className = 'bubble-wrapper ' + (isBot ? 'bubble-bot' : 'bubble-human');

  const label = document.createElement('div');
  label.className = 'bubble-label';
  label.textContent = speaker;

  const bubble = document.createElement('div');
  bubble.className = 'bubble';
  bubble.textContent = text;

  wrapper.appendChild(label);
  wrapper.appendChild(bubble);
  feed.appendChild(wrapper);
  feed.scrollTop = feed.scrollHeight;
}
```

- [ ] **Step 2: Update call sites**

Find the two `appendTranscript(` calls in the Wails event listeners and replace them:

```js
// transcript.final handler — change:
//   appendTranscript(speaker, ev.text, false);
// to:
  appendMessage(speaker, ev.text, false);

// voice.text handler — change:
//   appendTranscript(botName, ev.text, true);
// to:
  appendMessage(botName, ev.text, true);
```

- [ ] **Step 3: Remove the now-unused escapeHtml() function**

Delete the `function escapeHtml(s)` block — `appendMessage` uses `textContent` which is inherently safe.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/main.js
git commit -m "feat(js): replace appendTranscript with appendMessage using chat bubble DOM structure"
```

---

### Task 5: Update renderParticipants() to use pills

**Files:**
- Modify: `frontend/src/main.js`

The old function writes `<div class="participant">` into `#call-participants`. The new version writes `.participant-pill` divs — no `innerHTML`, no empty-hint (the status bar handles the alone state).

- [ ] **Step 1: Replace renderParticipants()**

Find and replace the `renderParticipants()` function:

```js
function renderParticipants() {
  const el = document.getElementById('call-participants');
  el.innerHTML = '';
  participants.forEach(name => {
    const pill = document.createElement('div');
    pill.className = 'participant-pill';
    pill.textContent = '👤 ' + name;
    el.appendChild(pill);
  });
}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/main.js
git commit -m "feat(js): render participants as pill chips in bottom row"
```

---

### Task 6: Update event handlers and resetCallScreen()

**Files:**
- Modify: `frontend/src/main.js`

Wire up `setBotState()` in all Wails event handlers. Update `resetCallScreen()` to clear `#call-feed` instead of `#call-transcript` and remove the `#call-participants` empty-hint. Update the `call-leave` handler to re-enable the button correctly.

- [ ] **Step 1: Update resetCallScreen()**

Find and replace `resetCallScreen()`:

```js
function resetCallScreen() {
  participants.clear();
  setBotState('joining');
  document.getElementById('call-feed').innerHTML =
    '<span class="empty-hint">Conversation will appear here…</span>';
  document.getElementById('call-participants').innerHTML = '';
  document.getElementById('call-warning').classList.add('hidden');
  document.getElementById('call-leave').disabled = false;
}
```

- [ ] **Step 2: Update call.bot_ready handler**

Find:
```js
window.runtime.EventsOn('call.bot_ready', async () => {
  setStatus('ready');
```
Replace `setStatus('ready')` with `setBotState('ready')`.

- [ ] **Step 3: Update participant.left handler**

Find the `participant.left` listener. Replace `if (participants.size === 0) setStatus('alone');` with:
```js
  if (participants.size === 0) setBotState('alone');
```

- [ ] **Step 4: Update participant.joined handler**

Find the `participant.joined` listener. Add a `setBotState('ready')` call after `renderParticipants()`:
```js
window.runtime.EventsOn('participant.joined', (ev) => {
  const p = ev.participant || {};
  if (p.id) participants.set(p.id, p.name || ev.name || 'Unknown');
  renderParticipants();
  setBotState('ready');
});
```

- [ ] **Step 5: Update voice.text handler to trigger responding state with debounce**

Find the `voice.text` listener. Replace it entirely:
```js
window.runtime.EventsOn('voice.text', (ev) => {
  if (ev.text) appendMessage(botName, ev.text, true);
  setBotState('responding');
  clearTimeout(respondingTimer);
  respondingTimer = setTimeout(() => setBotState('ready'), 2000);
});
```

- [ ] **Step 6: Update returnToJoin()**

Find `returnToJoin()`. The function resets the join button — no changes needed there. But confirm it no longer references `call-transcript`:
```bash
grep -n "call-transcript\|setStatus\|appendTranscript" frontend/src/main.js
```
Expected: no matches.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/main.js
git commit -m "feat(js): wire setBotState into all event handlers and update resetCallScreen"
```

---

### Task 7: Build and verify

**Files:** none (build only)

- [ ] **Step 1: Build**

```bash
/home/harikrishnan-c/go/bin/wails build -tags webkit2_41 2>&1
```

Expected: `Built '...AgentCall' in X.XXXs.` with no errors.

- [ ] **Step 2: Launch and verify joining state**

Run `build/bin/AgentCall`. Navigate to the join screen and click Join (or trigger any path that shows the call screen). Verify:
- Status bar shows amber dot + "Joining meeting..."
- Chat feed shows "Conversation will appear here…" hint
- Bottom row has Leave button on the right, no participant pills yet

- [ ] **Step 3: Verify responding state**

Once in a live call, when the bot speaks:
- Status bar background turns green-tinted
- Waveform bars animate
- Text reads "{botName} is responding…"
- After ~2 seconds of silence, reverts to pulsing dot + "…is listening"

- [ ] **Step 4: Final commit if any last tweaks were needed**

```bash
git add -p
git commit -m "fix(call-screen): tweak after visual verification"
```
