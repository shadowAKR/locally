# Local Agent

A desktop chat assistant for Linux, built with **Go + Wails**, that uses a model
running locally in **Ollama** to operate your laptop: run commands, read and
write files, find things, open apps and URLs, show notifications, and report
system health. Everything stays on your machine.

Anything that changes the system (writes, deletes, `sudo`, package installs,
service restarts, ...) is shown to you first with an **Approve / Deny** prompt,
unless you flip on *Auto-approve actions* in Settings (gear icon at the bottom of the sidebar, or Ctrl+,).

It also has its own **browser** (a separate Chrome profile you sign in to once)
so it can read Gmail, Outlook web and any website; **scheduled tasks** that run
at app start, daily at a time, or once at a date/time; and **chat history**
saved on disk.

## How it works

```
frontend/ (Vite, vanilla JS, dark UI)
   │  Wails bindings + "agent" events
app.go   ── App: SendMessage / Cancel / ResolveApproval / settings
   │
internal/agent     chat loop: model → tool calls → results → model …
internal/tools     the tool set + risk classifier (safe vs risky)
internal/ollama    streaming /api/chat client with tool calling
internal/config    ~/.config/localagent/settings.json
cmd/agentcli       terminal harness for the same agent (no GUI)
```

Tools the model can call (ten in total):

| Tool | Does | Gate |
|---|---|---|
| `run_shell` | runs a bash command (60 s default timeout) | read-only commands run freely, everything else asks |
| `read_file`, `list_directory`, `find_files`, `folder_sizes`, `system_info` | inspect; `folder_sizes` finds what uses disk space, `system_info` gives a labelled health snapshot | always safe |
| `write_file` | create / overwrite / append a file | asks |
| `trash` | move to trash via `gio trash` (recoverable) | asks |
| `open` | launch an app by name, or open a local file / folder | safe |
| `notify` | `notify-send` desktop notification | safe |
| `browser_open`, `browser_read` | open / re-read a page in the agent's Chrome profile; Gmail and Outlook inboxes come back as one line per message | safe |
| `browser_click`, `browser_type` | click by visible text, type into a field (never passwords) | asks |
| `schedule_task`, `list_scheduled_tasks`, `delete_scheduled_task` | manage scheduled prompts from chat | create/delete ask |

The shell classifier (`internal/tools/shell.go`) treats a command as read-only
only if every segment of the pipeline is on an allowlist (`ls`, `grep`, `df`,
`git status`, `systemctl status`, ...) and there is no redirection, `sudo`,
command substitution, or in-place edit. Everything else is "risky" and asks.

## Requirements

- Go 1.25+
- Node.js 18+ (for the frontend build)
- [Ollama](https://ollama.com) running locally with a model that supports
  tool calling, e.g. `ollama pull qwen3.5:2b` or `ollama pull llama3.2:3b`
- Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- Linux build deps (Ubuntu/Debian):

  ```bash
  sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev
  ```

## Build & run

```bash
# live-reload development (frontend + Go)
wails dev -tags webkit2_41

# production binary → build/bin/localagent
wails build -tags webkit2_41
./build/bin/localagent
```

The `webkit2_41` tag is needed on distros that ship WebKit2GTK 4.1 only
(Ubuntu 24.04+, Debian 13+, Fedora). On older distros with
`libwebkit2gtk-4.0-dev` you can drop it.

Only the frontend, in a normal browser with a scripted fake backend:

```bash
cd frontend && npm install && npm run dev
```

Terminal-only agent (same backend, no GUI; risky actions prompt on stdin):

```bash
go run ./cmd/agentcli "how much disk space do I have?"
go run ./cmd/agentcli -yes "create ~/Desktop/todo.md with three items"   # auto-approve
go run ./cmd/agentcli            # interactive REPL
```

## Packaging for Ubuntu (.deb)

The app icon lives in `build/appicon.svg`; `build/appicon.png` (1024 px) and
`build/linux/icons/localagent-*.png` are rendered from it and the PNG is
embedded as the window icon.

```bash
wails build -tags webkit2_41 -ldflags "-s -w"
./build/linux/package-deb.sh 0.1.0
```

This produces `build/bin/localagent_0.1.0_amd64.deb` containing the binary
(`/usr/bin/localagent`), a desktop entry, hicolor icons and the README. It
depends on `libgtk-3-0t64`, `libwebkit2gtk-4.1-0`, `libjavascriptcoregtk-4.1-0`
and `libglib2.0-bin` (for `gio trash`), and recommends `libnotify-bin` and a
Chromium-based browser for the email/web tools. Install with:

```bash
sudo apt install ./build/bin/localagent_0.1.0_amd64.deb
```

Ollama itself is not packaged; install it from https://ollama.com and pull a
tool-capable model (`ollama pull qwen3.5:2b`).

## Browser, email and sign-in

The first time the agent opens a site, a Chrome window appears using a
dedicated profile in `~/.local/share/localagent/browser-profile/`. Sign in to
Gmail / Outlook there once; the session is kept for later runs. The agent never
types passwords and will ask you to sign in when it lands on a login page.

If Google refuses the sign-in with "this browser may not be secure", sign in
once in that same window with the agent idle, or use an App Password.

## Scheduled tasks

Create tasks from the **Scheduled tasks** dialog (bottom of the sidebar, **New task**) or by asking the chat ("every day at
9 summarize my unread emails and notify me"). Kinds:

| Kind | Fires |
|---|---|
| `startup` | each time the app starts (turn on *Launch at login* so it starts with the laptop) |
| `daily` | every day at `HH:MM` |
| `once` | at a date/time; accepts `2026-10-31 09:00`, `tomorrow 9am`, `in 2 hours` |

Each run lands in its own chat (prefixed ⏰) and ends with a desktop
notification. Tasks are stored in `~/.config/localagent/schedules.json`. The app
must be running for schedules to fire; *Launch at login* writes an autostart
entry to `~/.config/autostart/localagent.desktop`.

## Chat history

Conversations are saved as JSON in `~/.local/share/localagent/chats/` and
listed in the sidebar. Delete one with the trash icon.

## Settings

Saved to `~/.config/localagent/settings.json`:

| Key | Default | Meaning |
|---|---|---|
| `ollamaUrl` | `http://localhost:11434` | Ollama server |
| `model` | first tool-capable model | model name |
| `autoApprove` | `false` | run risky actions without asking |
| `think` | `false` | enable the model's thinking mode (slower, more careful) |
| `numCtx` | `16384` | context window passed to Ollama |
| `temperature` | `0.2` | sampling temperature |

## Notes & limitations

- Small models (2B–4B) occasionally pick the wrong tool or need a retry; the
  agent tolerates up to 12 tool rounds per message and asks the model to
  adjust after errors. Larger models (7B+) are noticeably more reliable.
- Commands run non-interactively with no TTY; anything that waits for input
  will hit the timeout.
- The read-only allowlist is a heuristic, not a sandbox. `find -exec`, for
  example, is treated as read-only. If you want hard guarantees, keep
  *Auto-approve* off.
- Tested on Ubuntu 26.04 / GNOME. Other desktops work as long as `xdg-open`,
  `gio` and `notify-send` are present.
