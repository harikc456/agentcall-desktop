'use strict';

// ── State ──────────────────────────────────────────────────────────────────
const participants = new Map(); // id → name
let botName = 'Juno';
let respondingTimer = null;

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

  openAdvancedIfPopulated();
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
    const cfg = await window.go.main.App.GetConfig();
    await window.go.main.App.SaveConfig({ ...cfg, api_key: key });
    show('screen-join');
  } catch (e) {
    showError('setup-error', 'Could not save key: ' + e);
  }
});

// ── Join screen ───────────────────────────────────────────────────────────
// ── Advanced toggle ───────────────────────────────────────────────────────
document.getElementById('join-advanced-toggle').addEventListener('click', () => {
  const section = document.getElementById('join-advanced');
  const btn = document.getElementById('join-advanced-toggle');
  const open = section.classList.toggle('open');
  btn.textContent = open ? 'Advanced ▴' : 'Advanced ▾';
  btn.setAttribute('aria-expanded', open);
});

function openAdvancedIfPopulated() {
  const trigger = document.getElementById('join-trigger').value.trim();
  const ctx = document.getElementById('join-context').value.trim();
  if (trigger || ctx) {
    document.getElementById('join-advanced').classList.add('open');
    document.getElementById('join-advanced-toggle').textContent = 'Advanced ▴';
    document.getElementById('join-advanced-toggle').setAttribute('aria-expanded', 'true');
  }
}

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
  document.getElementById('setup-back').classList.remove('hidden');
  show('screen-setup');
});

// ── Gemini API key ────────────────────────────────────────────────────────

document.getElementById('setup-back').addEventListener('click', () => {
  show('screen-join');
});

async function refreshGeminiUI() {
  const cfg = await window.go.main.App.GetConfig();
  if (cfg.gemini_api_key) {
    document.getElementById('gemini-apikey').value = '';
    document.getElementById('gemini-apikey').placeholder = '●●●●●●●● (saved)';
    document.getElementById('gemini-save').classList.add('hidden');
    document.getElementById('gemini-clear').classList.remove('hidden');
  } else {
    document.getElementById('gemini-apikey').placeholder = 'AIza…';
    document.getElementById('gemini-save').classList.remove('hidden');
    document.getElementById('gemini-clear').classList.add('hidden');
  }
}

document.getElementById('gemini-save').addEventListener('click', async () => {
  const key = document.getElementById('gemini-apikey').value.trim();
  if (!key) { showError('gemini-key-error', 'Please enter a Gemini API key.'); return; }
  hideError('gemini-key-error');
  try {
    const cfg = await window.go.main.App.GetConfig();
    const newCfg = { ...cfg, gemini_api_key: key };
    await window.go.main.App.SaveConfig(newCfg);
    await refreshGeminiUI();
  } catch (e) {
    showError('gemini-key-error', 'Could not save key: ' + e);
  }
});

document.getElementById('gemini-clear').addEventListener('click', async () => {
  const cfg = await window.go.main.App.GetConfig();
  await window.go.main.App.SaveConfig({ ...cfg, gemini_api_key: '' });
  await refreshGeminiUI();
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
  setBotState('joining');
  document.getElementById('call-participants').innerHTML = '';
  document.getElementById('call-feed').innerHTML = '<span class="empty-hint">Conversation will appear here…</span>';
  document.getElementById('call-warning').classList.add('hidden');
  document.getElementById('call-leave').disabled = false;
}

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

function setStatus(state) {
  setBotState(state);
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

window.runtime.EventsOn('call.bot_ready', async () => {
  setStatus('ready');
  const status = await window.go.main.App.GetGeminiStatus();
  const chip = document.getElementById('gemini-chip');
  if (status.enabled) {
    chip.classList.remove('hidden');
  } else {
    chip.classList.add('hidden');
  }
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
