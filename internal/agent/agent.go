// Package agent runs the chat loop: send the conversation to the model, execute
// any tools it asks for (gated by an Approver), feed results back, repeat.
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/user"
	"runtime"
	"strings"
	"sync"
	"time"

	"localagent/internal/ollama"
	"localagent/internal/tools"
)

// Event is pushed to the UI as the agent works.
type Event struct {
	Type string `json:"type"` // status, delta, thinking, tool_call, approval, approval_resolved, tool_result, done, error

	// ChatID identifies the conversation the event belongs to (set by the app).
	ChatID string `json:"chatId,omitempty"`

	Text string `json:"text,omitempty"`

	ID       string `json:"id,omitempty"`
	Tool     string `json:"tool,omitempty"`
	Args     string `json:"args,omitempty"` // JSON-encoded arguments
	Risky    bool   `json:"risky,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Approved bool   `json:"approved,omitempty"`
	Output   string `json:"output,omitempty"`
	IsError  bool   `json:"isError,omitempty"`
	Duration int64  `json:"durationMs,omitempty"`
}

// Approver decides whether a risky tool call may run. Implementations block
// until the user answers or ctx is cancelled.
type Approver interface {
	Ask(ctx context.Context, id, tool string, args map[string]any, reason string) (bool, error)
}

// Config controls one Agent.
type Config struct {
	Model         string
	Think         bool
	AutoApprove   bool
	NumCtx        int
	MaxIterations int
	Temperature   float64
}

// Agent holds conversation state for one chat session.
type Agent struct {
	client   *ollama.Client
	tools    *tools.Registry
	approver Approver
	emit     func(Event)

	mu       sync.Mutex
	cfg      Config
	messages []ollama.Message
	seq      int
}

// New creates an agent. emit may be nil.
func New(client *ollama.Client, reg *tools.Registry, approver Approver, cfg Config, emit func(Event)) *Agent {
	if cfg.MaxIterations <= 0 {
		cfg.MaxIterations = 12
	}
	if cfg.NumCtx <= 0 {
		cfg.NumCtx = 16384
	}
	if emit == nil {
		emit = func(Event) {}
	}
	return &Agent{client: client, tools: reg, approver: approver, cfg: cfg, emit: emit}
}

// SetConfig replaces runtime configuration (model, approval mode, ...).
func (a *Agent) SetConfig(cfg Config) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if cfg.MaxIterations <= 0 {
		cfg.MaxIterations = a.cfg.MaxIterations
	}
	if cfg.NumCtx <= 0 {
		cfg.NumCtx = a.cfg.NumCtx
	}
	a.cfg = cfg
}

// Config returns the current configuration.
func (a *Agent) Config() Config {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cfg
}

// Reset clears the conversation history.
func (a *Agent) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.messages = nil
}

// Load replaces the conversation history (used when reopening a saved chat).
func (a *Agent) Load(msgs []ollama.Message) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.messages = append([]ollama.Message(nil), msgs...)
}

// History returns a copy of the conversation (without the system prompt).
func (a *Agent) History() []ollama.Message {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]ollama.Message, len(a.messages))
	copy(out, a.messages)
	return out
}

func (a *Agent) nextID(prefix string) string {
	a.seq++
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixMilli(), a.seq)
}

// Run processes one user message to completion, emitting events along the way.
func (a *Agent) Run(ctx context.Context, userText string) error {
	a.mu.Lock()
	cfg := a.cfg
	a.messages = append(a.messages, ollama.Message{Role: "user", Content: userText})
	a.mu.Unlock()

	if cfg.Model == "" {
		err := errors.New("no model selected")
		a.emit(Event{Type: "error", Text: err.Error()})
		return err
	}

	think := cfg.Think
	for iter := 0; iter < cfg.MaxIterations; iter++ {
		if ctx.Err() != nil {
			a.emit(Event{Type: "error", Text: "cancelled"})
			return ctx.Err()
		}
		a.emit(Event{Type: "status", Text: "thinking"})

		a.mu.Lock()
		msgs := append([]ollama.Message{{Role: "system", Content: a.systemPrompt()}}, a.messages...)
		a.mu.Unlock()

		req := ollama.ChatRequest{
			Model:     cfg.Model,
			Messages:  msgs,
			Tools:     a.tools.Definitions(),
			Think:     &think,
			KeepAlive: "15m",
			Options: map[string]any{
				"num_ctx":     cfg.NumCtx,
				"temperature": cfg.Temperature,
			},
		}

		reply, err := a.client.ChatStream(ctx, req, func(c ollama.ChatChunk) {
			if c.Message.Thinking != "" {
				a.emit(Event{Type: "thinking", Text: c.Message.Thinking})
			}
			if c.Message.Content != "" {
				a.emit(Event{Type: "delta", Text: c.Message.Content})
			}
		})
		if err != nil {
			if ctx.Err() != nil {
				a.emit(Event{Type: "error", Text: "cancelled"})
				return ctx.Err()
			}
			a.emit(Event{Type: "error", Text: err.Error()})
			return err
		}

		// Some small models emit the tool call as JSON text instead of a structured call.
		if len(reply.ToolCalls) == 0 {
			if tc, ok := parseInlineToolCall(reply.Content); ok {
				reply.ToolCalls = []ollama.ToolCall{tc}
				reply.Content = ""
			}
		}

		a.mu.Lock()
		a.messages = append(a.messages, reply)
		a.mu.Unlock()

		if len(reply.ToolCalls) == 0 {
			a.emit(Event{Type: "done"})
			return nil
		}

		for _, tc := range reply.ToolCalls {
			if ctx.Err() != nil {
				a.emit(Event{Type: "error", Text: "cancelled"})
				return ctx.Err()
			}
			result := a.executeTool(ctx, tc, cfg)
			a.mu.Lock()
			a.messages = append(a.messages, ollama.Message{Role: "tool", Content: result, ToolName: tc.Function.Name})
			a.mu.Unlock()
		}
	}

	a.emit(Event{Type: "error", Text: fmt.Sprintf("stopped after %d tool rounds without a final answer", cfg.MaxIterations)})
	a.emit(Event{Type: "done"})
	return nil
}

// executeTool runs one tool call, including the approval gate, and returns the
// text that goes back to the model.
func (a *Agent) executeTool(ctx context.Context, tc ollama.ToolCall, cfg Config) string {
	name := tc.Function.Name
	args := tc.Function.Arguments
	if args == nil {
		args = map[string]any{}
	}
	argsJSON, _ := json.Marshal(args)
	id := a.nextID("call")

	risk, reason := a.tools.Classify(name, args)
	risky := risk == tools.Risky
	a.emit(Event{Type: "tool_call", ID: id, Tool: name, Args: string(argsJSON), Risky: risky, Reason: reason})

	if risky && !cfg.AutoApprove {
		a.emit(Event{Type: "approval", ID: id, Tool: name, Args: string(argsJSON), Reason: reason})
		a.emit(Event{Type: "status", Text: "waiting for approval"})
		approved, err := a.approver.Ask(ctx, id, name, args, reason)
		if err != nil {
			a.emit(Event{Type: "approval_resolved", ID: id, Approved: false})
			out := "Tool call cancelled: " + err.Error()
			a.emit(Event{Type: "tool_result", ID: id, Tool: name, Output: out, IsError: true})
			return out
		}
		a.emit(Event{Type: "approval_resolved", ID: id, Approved: approved})
		if !approved {
			out := "The user declined this action. Do not retry it; ask the user how they would like to proceed or try a different approach."
			a.emit(Event{Type: "tool_result", ID: id, Tool: name, Output: out, IsError: true})
			return out
		}
	}

	a.emit(Event{Type: "status", Text: "running " + name})
	start := time.Now()
	out, err := a.tools.Run(ctx, name, args)
	dur := time.Since(start).Milliseconds()
	if err != nil {
		msg := "Error: " + err.Error()
		if strings.TrimSpace(out) != "" {
			msg += "\n" + out
		}
		a.emit(Event{Type: "tool_result", ID: id, Tool: name, Output: msg, IsError: true, Duration: dur})
		return msg
	}
	a.emit(Event{Type: "tool_result", ID: id, Tool: name, Output: out, Duration: dur})
	return out
}

// parseInlineToolCall recovers a tool call that the model wrote as plain JSON
// text, e.g. {"name":"run_shell","arguments":{"command":"ls"}}.
func parseInlineToolCall(content string) (ollama.ToolCall, bool) {
	s := strings.TrimSpace(content)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
		return ollama.ToolCall{}, false
	}
	var raw struct {
		Name       string         `json:"name"`
		Arguments  map[string]any `json:"arguments"`
		Parameters map[string]any `json:"parameters"`
	}
	if err := json.Unmarshal([]byte(s), &raw); err != nil || raw.Name == "" {
		return ollama.ToolCall{}, false
	}
	args := raw.Arguments
	if args == nil {
		args = raw.Parameters
	}
	return ollama.ToolCall{Function: ollama.ToolCallFunction{Name: raw.Name, Arguments: args}}, true
}

func (a *Agent) systemPrompt() string {
	home, _ := os.UserHomeDir()
	username := ""
	if u, err := user.Current(); err == nil {
		username = u.Username
	}
	host, _ := os.Hostname()
	osName := runtime.GOOS
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				osName = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), `"`)
			}
		}
	}
	desktop := os.Getenv("XDG_CURRENT_DESKTOP")

	return fmt.Sprintf(`You are a local assistant running on the user's laptop with permission to operate it through tools.

Environment:
- OS: %s (%s), desktop: %s
- User: %s, hostname: %s, home directory: %s
- Date/time: %s
- Shell: bash. Package manager: apt. Desktop tools available: xdg-open, gio, notify-send, nmcli, pactl, gsettings.

How to work:
1. When the user asks you to do or check something on the machine, DO IT with tools. Never describe what you would do, and never ask permission yourself; the app confirms risky actions with the user for you.
2. Pick the most specific tool:
   - health, battery, memory, disk, CPU, network questions -> system_info
   - "what is using disk space" / "which folder is big" -> folder_sizes on the home folder (or the folder named), then folder_sizes again on the biggest sub-folder to drill down. list_directory does NOT show folder sizes.
   - find a file by name -> find_files; see what is in a folder -> list_directory; read -> read_file; create/edit -> write_file; delete -> trash
   - launch an app or open a local file -> open
   - read emails, calendar, or ANY website content -> browser_open with the site URL, then read the result. You CAN read the user's Gmail and Outlook this way:
       Gmail inbox: browser_open https://mail.google.com/mail/u/0/#inbox   Outlook: browser_open https://outlook.office.com/mail/ (work) or https://outlook.live.com/mail/ (personal)
     The agent's browser keeps the user signed in. If the result says SIGN-IN page, tell the user to sign in in the browser window that opened, then try again later. Never type passwords.
     To open one email use browser_click with its subject text; use browser_type to search or fill forms; browser_read re-reads the current page.
   - run something later ("every day at 9", "tomorrow at 8am", "when the laptop starts", "remind me in 2 hours") -> schedule_task; list_scheduled_tasks / delete_scheduled_task to manage
   - everything else (install packages, processes, services, git, settings) -> run_shell
3. Use absolute paths or ~ paths. Never run interactive commands (vim, nano, less, top, ssh prompts); there is no terminal.
4. Tool outputs are labelled; copy numbers exactly as given and keep "used" and "free" straight. Do not invent values that are not in a tool result.
   Never claim you cannot access emails, websites or accounts: use browser_open. Only after a tool result reports a sign-in page should you ask the user to sign in.
5. If a tool fails, read the error, adjust, and try a different approach. Never repeat the exact same failing call.
6. If the user declines an action, do not attempt it again or work around it; ask what they would like instead.

How to answer (after the tools have run):
- Lead with the direct answer in one or two plain sentences, then the supporting details.
- Mention only what matters: flag problems (disk nearly full, low memory, high load) and skip values that are fine unless asked.
- When reporting several numbers, use a short markdown table with clear column names instead of bullet lists of bold labels.
- Use inline code only for paths, commands and filenames. No headings, no bold on every line, no emoji, no disclaimers.
- Keep it short: a typical answer is 3 to 8 lines.`,
		osName, runtime.GOARCH, desktop, username, host, home, time.Now().Format("Monday, 2 January 2006 15:04 MST"))
}
