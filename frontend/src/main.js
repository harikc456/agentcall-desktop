'use strict';

// ── State ──────────────────────────────────────────────────────────────────
const participants = new Map(); // id → name
let botName = 'Juno';

// ── Screen helpers ─────────────────────────────────────────────────────────
function show(id) {
  document.querySelectorAll('.screen').forEach(s => s.classList.add('hidden'));
  document.getElementById(id).classList.remove('hidden');
}

function showError(id, msg) {
  const el = document.getElementById(id);
  el.textContent = msg;
  el.classList.remove('hidden');
}

function hideError(id) {
  document.getElementById(id).classList.add('hidden');
}

// ── Startup ────────────────────────────────────────────────────────────────
window.addEventListener('load', async () => {
  const cfg = await window.go.main.App.GetConfig();

  if (!cfg.api_key) {
    await refreshGeminiUI();
    show('screen-setup');
    return;
  }

  // Pre-fill join form from saved config
  if (cfg.default_bot_name) document.getElementById('join-botname').value = cfg.default_bot_name;
  if (cfg.default_voice)    document.getElementById('join-voice').value    = cfg.default_voice;
  if (cfg.trigger_words)    document.getElementById('join-trigger').value  = cfg.trigger_words;
  if (cfg.context)          document.getElementById('join-context').value  = cfg.context;

  show('screen-join');
});

// ── Setup screen ──────────────────────────────────────────────────────────
document.getElementById('setup-save').addEventListener('click', async () => {
  hideError('setup-error');
  const key = document.getElementById('setup-apikey').value.trim();
  if (!key.startsWith('ak_ac_')) {
    showError('setup-error', 'API key must start with ak_ac_');
    return;
  }
  try {
    await window.go.main.App.SaveConfig({ api_key: key });
    show('screen-join');
  } catch (e) {
    showError('setup-error', 'Could not save key: ' + e);
  }
});

// ── Join screen ───────────────────────────────────────────────────────────
document.getElementById('join-button').addEventListener('click', async () => {
  hideError('join-error');

  const meetURL      = document.getElementById('join-url').value.trim();
  const name         = document.getElementById('join-botname').value.trim() || 'Juno';
  const triggerWords = document.getElementById('join-trigger').value.trim();
  const ctx          = document.getElementById('join-context').value.trim();
  const voice        = document.getElementById('join-voice').value;

  if (!meetURL) { showError('join-error', 'Please enter a meeting URL.'); return; }
  if (!meetURL.startsWith('http')) { showError('join-error', 'Please enter a valid meeting URL.'); return; }

  botName = name;

  // Save settings
  const cfg = await window.go.main.App.GetConfig();
  await window.go.main.App.SaveConfig({
    ...cfg,
    default_bot_name: name,
    default_voice:    voice,
    trigger_words:    triggerWords,
    context:          ctx,
  });

  const btn = document.getElementById('join-button');
  btn.disabled = true;
  btn.textContent = 'Joining…';

  try {
    await window.go.main.App.JoinMeeting(meetURL, name, triggerWords, ctx, voice);
    // Success — switch to call screen
    resetCallScreen();
    show('screen-call');
  } catch (e) {
    btn.disabled = false;
    btn.textContent = 'Join Meeting';
    showError('join-error', String(e));
  }
});

document.getElementById('join-settings').addEventListener('click', async () => {
  await refreshGeminiUI();
  show('screen-setup');
});

// ── Leave ─────────────────────────────────────────────────────────────────
document.getElementById('call-leave').addEventListener('click', async () => {
  document.getElementById('call-leave').disabled = true;
  try {
    await window.go.main.App.LeaveCall();
  } catch (_) {}
  returnToJoin();
});

// ── Call screen helpers ───────────────────────────────────────────────────
function resetCallScreen() {
  participants.clear();
  document.getElementById('call-status-dot').className = 'status-dot joining';
  document.getElementById('call-status-text').textContent = 'Joining meeting...';
  document.getElementById('call-participants').innerHTML = '<span class="empty-hint">Waiting for participants...</span>';
  document.getElementById('call-transcript').innerHTML   = '<span class="empty-hint">Transcript will appear here...</span>';
  document.getElementById('call-warning').classList.add('hidden');
  document.getElementById('call-leave').disabled = false;
}

function setStatus(state) {
  const dot  = document.getElementById('call-status-dot');
  const text = document.getElementById('call-status-text');
  dot.className = 'status-dot ' + state;
  text.textContent = {
    joining: 'Joining meeting...',
    ready:   botName + ' is in the meeting',
    alone:   'Alone in meeting — waiting for participants',
  }[state] || state;
}

function renderParticipants() {
  const el = document.getElementById('call-participants');
  if (participants.size === 0) {
    el.innerHTML = '<span class="empty-hint">No participants yet</span>';
    return;
  }
  el.innerHTML = '';
  participants.forEach(name => {
    const d = document.createElement('div');
    d.className = 'participant';
    d.textContent = '👤 ' + name;
    el.appendChild(d);
  });
}

function appendTranscript(speaker, text, isBot) {
  const feed = document.getElementById('call-transcript');
  // Remove empty-hint on first real line
  const hint = feed.querySelector('.empty-hint');
  if (hint) hint.remove();

  const line = document.createElement('div');
  line.className = 'transcript-line ' + (isBot ? 'bot' : 'human');
  line.innerHTML =
    `<span class="speaker">${escapeHtml(speaker)}:</span>` +
    `<span class="text">${escapeHtml(text)}</span>`;
  feed.appendChild(line);
  feed.scrollTop = feed.scrollHeight;
}

function escapeHtml(s) {
  return String(s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

function returnToJoin() {
  participants.clear();
  const btn = document.getElementById('join-button');
  btn.disabled = false;
  btn.textContent = 'Join Meeting';
  show('screen-join');
}

// ── Wails event listeners ─────────────────────────────────────────────────

// ── Gemini auth ───────────────────────────────────────────────────────────

async function refreshGeminiUI() {
  const status = await window.go.main.App.GetGeminiStatus();
  if (status.authenticated) {
    document.getElementById('gemini-signed-out').classList.add('hidden');
    document.getElementById('gemini-signed-in').classList.remove('hidden');
    document.getElementById('gemini-email').textContent = status.email;
  } else {
    document.getElementById('gemini-signed-out').classList.remove('hidden');
    document.getElementById('gemini-signed-in').classList.add('hidden');
    document.getElementById('gemini-email').textContent = '';
  }
}

document.getElementById('gemini-signin').addEventListener('click', async () => {
  const btn = document.getElementById('gemini-signin');
  hideError('gemini-auth-error');
  btn.disabled = true;
  btn.textContent = 'Opening browser…';
  try {
    await window.go.main.App.StartGeminiAuth();
    // UI update happens via the auth.gemini_ready Wails event
  } catch (e) {
    showError('gemini-auth-error', 'Could not start sign-in: ' + e);
    btn.disabled = false;
    btn.textContent = 'Sign in with Google';
  }
});

document.getElementById('gemini-signout').addEventListener('click', async () => {
  await window.go.main.App.SignOutGemini();
});

window.runtime.EventsOn('auth.gemini_ready', async () => {
  const btn = document.getElementById('gemini-signin');
  btn.disabled = false;
  btn.textContent = 'Sign in with Google';
  await refreshGeminiUI();
});

window.runtime.EventsOn('auth.gemini_error', (ev) => {
  const btn = document.getElementById('gemini-signin');
  btn.disabled = false;
  btn.textContent = 'Sign in with Google';
  showError('gemini-auth-error', (ev && ev.error) || 'Sign-in failed.');
});

window.runtime.EventsOn('auth.gemini_signed_out', async () => {
  await refreshGeminiUI();
});

// ── Gemini active chip (call screen) ─────────────────────────────────────

async function updateGeminiChip() {
  const status = await window.go.main.App.GetGeminiStatus();
  const chip = document.getElementById('gemini-chip');
  if (status.authenticated) {
    chip.classList.remove('hidden');
  } else {
    chip.classList.add('hidden');
  }
}

window.runtime.EventsOn('call.bot_ready', async () => {
  setStatus('ready');
  await updateGeminiChip();
});

window.runtime.EventsOn('participant.joined', (ev) => {
  const p = ev.participant || {};
  if (p.id) participants.set(p.id, p.name || ev.name || 'Unknown');
  renderParticipants();
});

window.runtime.EventsOn('participant.left', (ev) => {
  const p = ev.participant || {};
  if (p.id) participants.delete(p.id);
  renderParticipants();
  if (participants.size === 0) setStatus('alone');
});

window.runtime.EventsOn('transcript.final', (ev) => {
  const speaker = ev.speaker ? ev.speaker.name : 'Unknown';
  if (ev.text) appendTranscript(speaker, ev.text, false);
});

window.runtime.EventsOn('voice.text', (ev) => {
  if (ev.text) appendTranscript(botName, ev.text, true);
});

window.runtime.EventsOn('call.ended', () => {
  returnToJoin();
});

window.runtime.EventsOn('call.credits_low', (ev) => {
  const warn = document.getElementById('call-warning');
  const mins = ev.estimated_minutes_remaining || '?';
  warn.textContent = `Credits low — approximately ${mins} minutes remaining. Add credits at app.agentcall.dev/add-credits`;
  warn.classList.remove('hidden');
});
