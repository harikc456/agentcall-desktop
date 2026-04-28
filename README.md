# AgentCall Desktop

A cross-platform desktop app that lets non-technical users join video meetings (Google Meet, Teams, Zoom) as an AI bot — no Python, no terminal, no dependencies.

The bot joins with a voice, listens to the meeting, and responds to questions based on the context you provide. Powered by [AgentCall](https://agentcall.dev).

---

## What You Need

- An AgentCall API key — get one at [app.agentcall.dev/api-keys](https://app.agentcall.dev/api-keys)
- Go 1.21 or newer — [golang.org/dl](https://golang.org/dl)
- Wails CLI v2 — installed in the step below

---

## Build Instructions

### macOS

**Prerequisites**

```bash
# Install Go (if not already installed)
brew install go

# Install Wails
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

**Build**

```bash
git clone <repo-url>
cd agentcall-desktop

wails build
```

The app is produced at `build/bin/AgentCall.app`.

**Install**

Drag `AgentCall.app` to your Applications folder and double-click to open.

> **Apple Silicon (M1/M2/M3):** Wails builds a native arm64 binary by default. To build a universal binary (runs on both Intel and Apple Silicon):
> ```bash
> wails build -platform darwin/universal
> ```

---

### Windows

**Prerequisites**

1. Install Go from [golang.org/dl](https://golang.org/dl) — use the `.msi` installer
2. Install [WebView2 Runtime](https://developer.microsoft.com/en-us/microsoft-edge/webview2/) — required by Wails (most Windows 11 machines already have it)
3. Open PowerShell and install Wails:

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

**Build**

```powershell
git clone <repo-url>
cd agentcall-desktop

wails build
```

The app is produced at `build\bin\AgentCall.exe`.

**Run**

Double-click `AgentCall.exe` — no installer needed.

> **Windows Defender warning:** The first time you run the unsigned binary, Windows may show a SmartScreen warning. Click "More info" → "Run anyway".

---

### Linux

**Prerequisites**

```bash
# Install Go
wget https://go.dev/dl/go1.23.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.23.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# Install Wails
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Install required system libraries (Ubuntu / Debian)
sudo apt-get update
sudo apt-get install -y \
  libgtk-3-dev \
  libwebkit2gtk-4.1-dev \
  build-essential \
  pkg-config
```

> **webkit2gtk version:** Ubuntu 22.04+ ships `libwebkit2gtk-4.1`. Older distros may have `libwebkit2gtk-4.0`. If `4.1` is not available, install `libwebkit2gtk-4.0-dev` and drop the `-tags webkit2_41` flag from the build command below.

**Build**

```bash
git clone <repo-url>
cd agentcall-desktop

# Ubuntu 22.04+ (webkit2gtk-4.1)
wails build -tags webkit2_41

# Ubuntu 20.04 / older (webkit2gtk-4.0)
wails build
```

The binary is produced at `build/bin/AgentCall`.

**Run**

```bash
./build/bin/AgentCall
```

Or double-click it in your file manager.

---

## First Launch

1. The app opens to the **Setup screen** — enter your AgentCall API key (`ak_ac_...`)
2. Click **Save & Continue**
3. You land on the **Join screen** — your settings are saved and pre-filled next time

---

## Joining a Meeting

1. Paste a meeting URL (Google Meet, Zoom, or Teams)
2. Set a **Bot Name** — the name participants see in the meeting (e.g. `Juno`)
3. Pick a **Voice** — Heart, Bella, Echo, or Eric
4. Set **Trigger Words** — comma-separated words the bot listens for (e.g. `juno, june`)
5. Optionally add **Context** — what the bot should know:
   ```
   You are a meeting assistant for Acme Corp.
   Q3 revenue was $2.4M, up 15% year over year.
   The roadmap priorities are: payments, mobile, and API v2.
   ```
6. Click **Join Meeting**

The bot joins in 30–90 seconds. Once it's in, participants can speak its name and ask questions based on the context you provided.

---

## Leaving a Meeting

Click **Leave Meeting** in the app. The bot leaves immediately and billing stops.

The bot also auto-leaves if:
- It's alone for 2 minutes
- The meeting has been silent for 5 minutes
- The maximum call duration is reached

---

## API Key

Only one key is needed: your **AgentCall API key** (`ak_ac_...`).

- Get it at [app.agentcall.dev/api-keys](https://app.agentcall.dev/api-keys)
- Stored at `~/.agentcall/config.json` (shared with the AgentCall CLI)
- To change it: click **⚙ Change API Key** on the Join screen

---

## Live Development Mode

To run the app with hot-reload during development:

```bash
# macOS / Windows
wails dev

# Linux (Ubuntu 22.04+)
wails dev -tags webkit2_41
```

This opens the app window and automatically rebuilds when you edit Go or frontend files.

---

## Troubleshooting

**App shows blank screen on Linux**
Install the WebKit dev libraries:
```bash
sudo apt-get install libwebkit2gtk-4.1-dev
```

**"API key must start with ak_ac_"**
Make sure you're copying the full key from [app.agentcall.dev/api-keys](https://app.agentcall.dev/api-keys).

**Bot joins but doesn't respond**
Check that your trigger words match what you said. Speech-to-text can mishear names — add common variants (e.g. `juno, june, you know`).

**Build fails on Windows: "WebView2 not found"**
Install the WebView2 Runtime: [microsoft.com/en-us/microsoft-edge/webview2](https://developer.microsoft.com/en-us/microsoft-edge/webview2/)

**Credits running low**
Add credits at [app.agentcall.dev/add-credits](https://app.agentcall.dev/add-credits). The app shows a warning banner when credits are low.
