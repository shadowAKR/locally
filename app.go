package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"localagent/internal/agent"
	"localagent/internal/config"
	"localagent/internal/ollama"
	"localagent/internal/schedule"
	"localagent/internal/store"
	"localagent/internal/tools"
)

// eventName is the single Wails event channel the frontend subscribes to.
const eventName = "agent"

// session is one open conversation: its saved chat plus the agent holding it.
type session struct {
	chat  *store.Chat
	agent *agent.Agent
}

// App is the Wails-bound backend. Its exported methods are callable from JS.
type App struct {
	ctx context.Context

	mu        sync.Mutex
	settings  config.Settings
	client    *ollama.Client
	registry  *tools.Registry
	store     *store.Store
	sched     *schedule.Scheduler
	current   *session
	running   bool
	runningID string
	cancelRun context.CancelFunc

	gate *approvalGate
}

// approvalGate implements agent.Approver by blocking on a per-call channel that
// the UI releases through App.ResolveApproval. It is a separate unexported type
// so Wails does not expose Ask to the frontend.
type approvalGate struct {
	mu      sync.Mutex
	pending map[string]chan bool
}

// Ask blocks until the frontend resolves this id or ctx is cancelled.
func (g *approvalGate) Ask(ctx context.Context, id, tool string, args map[string]any, reason string) (bool, error) {
	ch := make(chan bool, 1)
	g.mu.Lock()
	g.pending[id] = ch
	g.mu.Unlock()
	defer func() {
		g.mu.Lock()
		delete(g.pending, id)
		g.mu.Unlock()
	}()
	select {
	case ok := <-ch:
		return ok, nil
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

func (g *approvalGate) resolve(id string, approved bool) {
	g.mu.Lock()
	ch, ok := g.pending[id]
	g.mu.Unlock()
	if ok {
		select {
		case ch <- approved:
		default:
		}
	}
}

// NewApp creates a new App application struct.
func NewApp() *App {
	return &App{gate: &approvalGate{pending: map[string]chan bool{}}}
}

// startup is called when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.settings = config.Load()
	a.client = ollama.New(a.settings.OllamaURL)

	var err error
	if a.store, err = store.Open(""); err != nil {
		runtime.LogErrorf(ctx, "chat store: %v", err)
	}
	if a.sched, err = schedule.Load("", a.runScheduled); err != nil {
		runtime.LogErrorf(ctx, "schedules: %v", err)
		a.sched, _ = schedule.Load(filepath.Join(os.TempDir(), "localagent-schedules.json"), a.runScheduled)
	}
	a.sched.OnChange(func() { a.emit(agent.Event{Type: "schedules_changed"}) })

	a.registry = tools.NewRegistry()
	for _, t := range scheduleTools(a.sched) {
		a.registry.Add(t)
	}

	// Pick a default model if none is configured yet.
	if a.settings.Model == "" {
		if models, err := a.client.ListModels(ctx); err == nil {
			for _, m := range models {
				if m.SupportsTools() {
					a.settings.Model = m.Name
					break
				}
			}
			if a.settings.Model == "" && len(models) > 0 {
				a.settings.Model = models[0].Name
			}
		}
	}
	a.current = a.newSession(store.NewChat("user"))
	a.sched.Start()
}

// shutdown is called when the window closes.
func (a *App) shutdown(ctx context.Context) {
	a.Cancel()
	if a.sched != nil {
		a.sched.Stop()
	}
	tools.CloseBrowser()
}

func (a *App) agentConfig() agent.Config {
	return agent.Config{
		Model:       a.settings.Model,
		Think:       a.settings.Think,
		AutoApprove: a.settings.AutoApprove,
		NumCtx:      a.settings.NumCtx,
		Temperature: a.settings.Temperature,
	}
}

func (a *App) newSession(chat *store.Chat) *session {
	id := chat.ID
	ag := agent.New(a.client, a.registry, a.gate, a.agentConfig(), func(ev agent.Event) {
		ev.ChatID = id
		a.emit(ev)
	})
	ag.Load(chat.Messages)
	return &session{chat: chat, agent: ag}
}

func (a *App) emit(ev agent.Event) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, eventName, ev)
	}
}

// tryStart claims the single "running" slot. Returns a context and release func.
func (a *App) tryStart(parent context.Context, chatID string) (context.Context, func(), error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.running {
		return nil, nil, errors.New("the agent is still working on another message")
	}
	ctx, cancel := context.WithCancel(parent)
	a.running = true
	a.runningID = chatID
	a.cancelRun = cancel
	release := func() {
		cancel()
		a.mu.Lock()
		a.running = false
		a.runningID = ""
		a.cancelRun = nil
		a.mu.Unlock()
	}
	return ctx, release, nil
}

// ResolveApproval is called by the UI when the user approves or denies a tool call.
func (a *App) ResolveApproval(id string, approved bool) {
	a.gate.resolve(id, approved)
}

// ---- Chat ----

// SendMessage starts processing a user message in the current chat.
func (a *App) SendMessage(text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return errors.New("empty message")
	}
	a.mu.Lock()
	sess := a.current
	a.mu.Unlock()
	ctx, release, err := a.tryStart(a.ctx, sess.chat.ID)
	if err != nil {
		return err
	}
	if sess.chat.Title == "" {
		sess.chat.Title = store.TitleFrom(text)
	}
	go func() {
		defer release()
		_ = sess.agent.Run(ctx, text)
		a.persist(sess)
	}()
	return nil
}

func (a *App) persist(sess *session) {
	sess.chat.Messages = sess.agent.History()
	if a.store != nil {
		if err := a.store.Save(sess.chat); err != nil {
			runtime.LogErrorf(a.ctx, "save chat: %v", err)
		}
	}
	a.emit(agent.Event{Type: "chats_changed", ChatID: sess.chat.ID})
}

// Cancel stops the current run, if any.
func (a *App) Cancel() {
	a.mu.Lock()
	cancel := a.cancelRun
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// IsRunning reports whether a message is being processed.
func (a *App) IsRunning() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.running
}

// ChatView is a full conversation for rendering.
type ChatView struct {
	ID       string           `json:"id"`
	Title    string           `json:"title"`
	Source   string           `json:"source"`
	Messages []ollama.Message `json:"messages"`
}

// NewChat starts a fresh conversation and returns its id.
func (a *App) NewChat() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.current != nil && len(a.current.chat.Messages) == 0 && !a.running {
		return a.current.chat.ID // already on an empty chat
	}
	a.current = a.newSession(store.NewChat("user"))
	return a.current.chat.ID
}

// CurrentChat returns the conversation currently open.
func (a *App) CurrentChat() ChatView {
	a.mu.Lock()
	defer a.mu.Unlock()
	c := a.current.chat
	return ChatView{ID: c.ID, Title: c.Title, Source: c.Source, Messages: a.current.agent.History()}
}

// ListChats returns saved conversations, newest first.
func (a *App) ListChats() ([]store.Summary, error) {
	if a.store == nil {
		return nil, errors.New("chat store unavailable")
	}
	return a.store.List()
}

// LoadChat opens a saved conversation and makes it current.
func (a *App) LoadChat(id string) (ChatView, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.running && a.runningID != id {
		return ChatView{}, errors.New("wait for the current task to finish before switching chats")
	}
	if a.current != nil && a.current.chat.ID == id {
		c := a.current.chat
		return ChatView{ID: c.ID, Title: c.Title, Source: c.Source, Messages: a.current.agent.History()}, nil
	}
	c, err := a.store.Load(id)
	if err != nil {
		return ChatView{}, err
	}
	a.current = a.newSession(c)
	return ChatView{ID: c.ID, Title: c.Title, Source: c.Source, Messages: c.Messages}, nil
}

// DeleteChat removes a saved conversation.
func (a *App) DeleteChat(id string) error {
	a.mu.Lock()
	isCurrent := a.current != nil && a.current.chat.ID == id
	if a.running && a.runningID == id {
		a.mu.Unlock()
		return errors.New("that chat is still running")
	}
	if isCurrent {
		a.current = a.newSession(store.NewChat("user"))
	}
	a.mu.Unlock()
	return a.store.Delete(id)
}

// ---- Schedules ----

// runScheduled is the scheduler's Runner: it executes a task's prompt in a new chat.
func (a *App) runScheduled(t schedule.Task) (string, error) {
	chat := store.NewChat("schedule")
	chat.Title = "⏰ " + t.Name
	sess := a.newSession(chat)

	// Wait (up to 10 minutes) for the user's current task to finish.
	var ctx context.Context
	var release func()
	deadline := time.Now().Add(10 * time.Minute)
	for {
		var err error
		ctx, release, err = a.tryStart(a.ctx, chat.ID)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			return "", errors.New("agent was busy; scheduled task skipped")
		}
		time.Sleep(5 * time.Second)
	}
	defer release()
	ctx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()

	a.emit(agent.Event{Type: "chat_started", ChatID: chat.ID, Text: t.Name, Tool: "schedule"})
	prompt := fmt.Sprintf("[Scheduled task \"%s\", running automatically at %s]\n%s", t.Name, time.Now().Format("15:04 on Mon 2 Jan"), t.Prompt)
	err := sess.agent.Run(ctx, prompt)
	a.persist(sess)

	// Desktop notification with the gist of the answer.
	summary := "Finished."
	msgs := sess.agent.History()
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "assistant" && strings.TrimSpace(msgs[i].Content) != "" {
			summary = strings.Join(strings.Fields(msgs[i].Content), " ")
			if len(summary) > 220 {
				summary = summary[:217] + "..."
			}
			break
		}
	}
	if err != nil {
		summary = "Failed: " + err.Error()
	}
	_ = exec.Command("notify-send", "Local Agent: "+t.Name, summary).Run()
	return chat.ID, err
}

// ListSchedules returns scheduled tasks with their next run time.
func (a *App) ListSchedules() []schedule.Task {
	return a.sched.List()
}

// AddSchedule creates a task. kind: startup | daily | once. timeOfDay is HH:MM for
// daily; when is a date/time phrase for once ("2026-10-31 09:00", "tomorrow 9am").
func (a *App) AddSchedule(name, prompt, kind, timeOfDay, when string) (schedule.Task, error) {
	var at time.Time
	if schedule.Kind(kind) == schedule.Once {
		var err error
		if at, err = schedule.ParseWhen(when, time.Now()); err != nil {
			return schedule.Task{}, err
		}
	}
	return a.sched.Add(name, prompt, schedule.Kind(kind), timeOfDay, at)
}

// SetScheduleEnabled toggles a task.
func (a *App) SetScheduleEnabled(id string, enabled bool) error {
	return a.sched.SetEnabled(id, enabled)
}

// DeleteSchedule removes a task.
func (a *App) DeleteSchedule(id string) error { return a.sched.Delete(id) }

// RunScheduleNow fires a task immediately.
func (a *App) RunScheduleNow(id string) error { return a.sched.RunNow(id) }

// ---- Launch at login ----

func autostartFile() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "autostart", "localagent.desktop")
}

// GetLaunchAtLogin reports whether the autostart entry exists.
func (a *App) GetLaunchAtLogin() bool {
	_, err := os.Stat(autostartFile())
	return err == nil
}

// SetLaunchAtLogin creates or removes the autostart entry for this binary.
func (a *App) SetLaunchAtLogin(enabled bool) error {
	path := autostartFile()
	if !enabled {
		err := os.Remove(path)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	content := fmt.Sprintf("[Desktop Entry]\nType=Application\nName=Local Agent\nComment=Local AI assistant for this laptop\nExec=%s\nIcon=utilities-terminal\nTerminal=false\nX-GNOME-Autostart-enabled=true\n", exe)
	return os.WriteFile(path, []byte(content), 0o644)
}

// ---- Models & settings ----

// ModelEntry is what the UI shows in the model picker.
type ModelEntry struct {
	Name          string `json:"name"`
	Size          string `json:"size"`
	ParameterSize string `json:"parameterSize"`
	SupportsTools bool   `json:"supportsTools"`
}

// ListModels returns the models available on the Ollama server.
func (a *App) ListModels() ([]ModelEntry, error) {
	models, err := a.client.ListModels(a.ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ModelEntry, 0, len(models))
	for _, m := range models {
		out = append(out, ModelEntry{
			Name:          m.Name,
			Size:          fmt.Sprintf("%.1f GB", float64(m.Size)/1e9),
			ParameterSize: m.Details.ParameterSize,
			SupportsTools: m.SupportsTools(),
		})
	}
	return out, nil
}

// Status describes the backend connection for the UI header.
type Status struct {
	Connected     bool   `json:"connected"`
	OllamaVersion string `json:"ollamaVersion"`
	Error         string `json:"error,omitempty"`
	Model         string `json:"model"`
}

// GetStatus pings Ollama.
func (a *App) GetStatus() Status {
	v, err := a.client.Version(a.ctx)
	s := Status{Connected: err == nil, OllamaVersion: v, Model: a.settings.Model}
	if err != nil {
		s.Error = err.Error()
	}
	return s
}

// GetSettings returns the current settings.
func (a *App) GetSettings() config.Settings {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.settings
}

// SaveSettings persists settings and applies them to the current agent.
func (a *App) SaveSettings(s config.Settings) error {
	if s.OllamaURL == "" {
		s.OllamaURL = config.Default().OllamaURL
	}
	if s.NumCtx <= 0 {
		s.NumCtx = config.Default().NumCtx
	}
	a.mu.Lock()
	urlChanged := s.OllamaURL != a.settings.OllamaURL
	a.settings = s
	if urlChanged {
		a.client = ollama.New(s.OllamaURL)
		a.current = a.newSession(a.current.chat)
	} else if a.current != nil {
		a.current.agent.SetConfig(a.agentConfig())
	}
	a.mu.Unlock()
	return config.Save(s)
}

// StartOllama tries to launch the Ollama server if it is installed but not running.
func (a *App) StartOllama() error {
	if _, err := exec.LookPath("ollama"); err != nil {
		return errors.New("ollama is not installed; see https://ollama.com/download")
	}
	cmd := exec.Command("ollama", "serve")
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait() //nolint:errcheck
	return nil
}

// ToolNames lists the tools the model can use (for the UI help panel).
func (a *App) ToolNames() []string {
	return a.registry.Names()
}

// Paths tells the UI where data lives.
func (a *App) Paths() map[string]string {
	out := map[string]string{"schedules": schedule.DefaultPath()}
	if a.store != nil {
		out["chats"] = a.store.Path()
	}
	return out
}
