// Package schedule stores and fires scheduled agent prompts: at app start,
// daily at a fixed time, or once at a specific date and time.
package schedule

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Kind is when a task fires.
type Kind string

const (
	// Startup runs every time the app starts.
	Startup Kind = "startup"
	// Daily runs every day at Time (HH:MM, local).
	Daily Kind = "daily"
	// Once runs a single time at At.
	Once Kind = "once"
)

// Task is one scheduled prompt.
type Task struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Prompt    string    `json:"prompt"`
	Kind      Kind      `json:"kind"`
	Time      string    `json:"time,omitempty"` // daily: "HH:MM"
	At        time.Time `json:"at,omitempty"`   // once: absolute time
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"createdAt"`
	LastRun   time.Time `json:"lastRun,omitempty"`
	LastChat  string    `json:"lastChat,omitempty"`
	NextRun   time.Time `json:"nextRun,omitempty"` // computed, not persisted meaningfully
}

// Runner executes a task's prompt. It returns the chat id the run was saved to.
type Runner func(t Task) (chatID string, err error)

// Scheduler owns the task list and the ticking loop.
type Scheduler struct {
	mu     sync.Mutex
	path   string
	tasks  []Task
	runner Runner
	stop   chan struct{}
	onEdit func()
}

// DefaultPath is ~/.config/localagent/schedules.json.
func DefaultPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "localagent", "schedules.json")
}

// Load reads the task file (missing file is fine).
func Load(path string, runner Runner) (*Scheduler, error) {
	if path == "" {
		path = DefaultPath()
	}
	s := &Scheduler{path: path, runner: runner, stop: make(chan struct{})}
	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &s.tasks); err != nil {
			return nil, fmt.Errorf("schedules.json is corrupt: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return s, nil
}

// OnChange registers a callback invoked after any edit (for UI refresh).
func (s *Scheduler) OnChange(f func()) { s.onEdit = f }

func (s *Scheduler) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.tasks, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Scheduler) changed() {
	if s.onEdit != nil {
		go s.onEdit()
	}
}

// List returns tasks with NextRun filled in, soonest first.
func (s *Scheduler) List() []Task {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	out := make([]Task, len(s.tasks))
	for i, t := range s.tasks {
		t.NextRun = nextRun(t, now)
		out[i] = t
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].NextRun.IsZero() != out[j].NextRun.IsZero() {
			return !out[i].NextRun.IsZero()
		}
		return out[i].NextRun.Before(out[j].NextRun)
	})
	return out
}

// Add validates and stores a new task.
func (s *Scheduler) Add(name, prompt string, kind Kind, timeOfDay string, at time.Time) (Task, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return Task{}, errors.New("prompt is required")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = prompt
		if len(name) > 40 {
			name = name[:37] + "..."
		}
	}
	t := Task{
		ID:        fmt.Sprintf("task-%d", time.Now().UnixNano()),
		Name:      name,
		Prompt:    prompt,
		Kind:      kind,
		Enabled:   true,
		CreatedAt: time.Now(),
	}
	switch kind {
	case Startup:
	case Daily:
		hhmm, err := normalizeTime(timeOfDay)
		if err != nil {
			return Task{}, err
		}
		t.Time = hhmm
	case Once:
		if at.IsZero() {
			return Task{}, errors.New("a date and time are required for a one-off task")
		}
		if at.Before(time.Now()) {
			return Task{}, fmt.Errorf("%s is in the past", at.Format("2006-01-02 15:04"))
		}
		t.At = at
	default:
		return Task{}, fmt.Errorf("unknown schedule kind %q (use startup, daily or once)", kind)
	}
	s.mu.Lock()
	s.tasks = append(s.tasks, t)
	err := s.saveLocked()
	s.mu.Unlock()
	s.changed()
	return t, err
}

// SetEnabled toggles a task.
func (s *Scheduler) SetEnabled(id string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.tasks {
		if s.tasks[i].ID == id {
			s.tasks[i].Enabled = enabled
			err := s.saveLocked()
			s.changed()
			return err
		}
	}
	return errors.New("task not found")
}

// Delete removes a task.
func (s *Scheduler) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.tasks {
		if s.tasks[i].ID == id {
			s.tasks = append(s.tasks[:i], s.tasks[i+1:]...)
			err := s.saveLocked()
			s.changed()
			return err
		}
	}
	return errors.New("task not found")
}

// Get returns a task by id.
func (s *Scheduler) Get(id string) (Task, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.tasks {
		if t.ID == id {
			return t, true
		}
	}
	return Task{}, false
}

// Start runs startup tasks, then ticks every 20 seconds for due tasks until Stop.
func (s *Scheduler) Start() {
	go func() {
		time.Sleep(3 * time.Second) // let the UI connect first
		for _, t := range s.List() {
			if t.Enabled && t.Kind == Startup {
				s.fire(t)
			}
		}
		tick := time.NewTicker(20 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-s.stop:
				return
			case now := <-tick.C:
				for _, t := range s.List() {
					if t.Enabled && !t.NextRun.IsZero() && !t.NextRun.After(now) && t.Kind != Startup {
						s.fire(t)
					}
				}
			}
		}
	}()
}

// Stop ends the ticking loop.
func (s *Scheduler) Stop() { close(s.stop) }

// RunNow fires a task immediately in the background.
func (s *Scheduler) RunNow(id string) error {
	t, ok := s.Get(id)
	if !ok {
		return errors.New("task not found")
	}
	go s.fire(t)
	return nil
}

func (s *Scheduler) fire(t Task) {
	// Mark the run first so a slow run is not fired twice by the next tick.
	s.mu.Lock()
	for i := range s.tasks {
		if s.tasks[i].ID == t.ID {
			s.tasks[i].LastRun = time.Now()
			if t.Kind == Once {
				s.tasks[i].Enabled = false
			}
		}
	}
	_ = s.saveLocked()
	s.mu.Unlock()
	s.changed()

	chatID, _ := s.runner(t)

	s.mu.Lock()
	for i := range s.tasks {
		if s.tasks[i].ID == t.ID {
			s.tasks[i].LastChat = chatID
		}
	}
	_ = s.saveLocked()
	s.mu.Unlock()
	s.changed()
}

// nextRun computes when a task fires next; zero for startup/disabled/done.
func nextRun(t Task, now time.Time) time.Time {
	if !t.Enabled {
		return time.Time{}
	}
	switch t.Kind {
	case Daily:
		hh, mm, err := parseHHMM(t.Time)
		if err != nil {
			return time.Time{}
		}
		next := time.Date(now.Year(), now.Month(), now.Day(), hh, mm, 0, 0, now.Location())
		// If it already ran today (or the slot has passed), move to tomorrow.
		if !next.After(now) || (!t.LastRun.IsZero() && sameDay(t.LastRun, next)) {
			if !next.After(now) || sameDay(t.LastRun, next) {
				next = next.Add(24 * time.Hour)
			}
		}
		return next
	case Once:
		if !t.LastRun.IsZero() {
			return time.Time{}
		}
		return t.At
	}
	return time.Time{}
}

func sameDay(a, b time.Time) bool {
	y1, m1, d1 := a.Date()
	y2, m2, d2 := b.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

func parseHHMM(s string) (int, int, error) {
	var hh, mm int
	if _, err := fmt.Sscanf(strings.TrimSpace(s), "%d:%d", &hh, &mm); err != nil || hh < 0 || hh > 23 || mm < 0 || mm > 59 {
		return 0, 0, fmt.Errorf("time must be HH:MM in 24-hour format, got %q", s)
	}
	return hh, mm, nil
}

func normalizeTime(s string) (string, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	// Accept "9am", "9:30 pm", "21:05".
	pm := strings.HasSuffix(s, "pm")
	am := strings.HasSuffix(s, "am")
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(s, "pm"), "am"))
	if !strings.Contains(s, ":") {
		s += ":00"
	}
	hh, mm, err := parseHHMM(s)
	if err != nil {
		return "", err
	}
	if pm && hh < 12 {
		hh += 12
	}
	if am && hh == 12 {
		hh = 0
	}
	return fmt.Sprintf("%02d:%02d", hh, mm), nil
}

// ParseWhen interprets a one-off time like "2026-10-31 09:00", "tomorrow 9am" or "today 18:30".
func ParseWhen(s string, now time.Time) (time.Time, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	for _, layout := range []string{"2006-01-02 15:04", "2006-01-02T15:04", "2006-01-02 3:04pm", "2006-01-02 3pm", time.RFC3339} {
		if t, err := time.ParseInLocation(layout, s, now.Location()); err == nil {
			return t, nil
		}
	}
	day := now
	rest := s
	switch {
	case strings.HasPrefix(s, "tomorrow"):
		day = now.Add(24 * time.Hour)
		rest = strings.TrimSpace(strings.TrimPrefix(s, "tomorrow"))
	case strings.HasPrefix(s, "today"):
		rest = strings.TrimSpace(strings.TrimPrefix(s, "today"))
	case strings.HasPrefix(s, "in "):
		d, err := time.ParseDuration(strings.ReplaceAll(strings.TrimPrefix(s, "in "), " ", ""))
		if err != nil {
			// allow "in 2 hours", "in 30 minutes"
			var n int
			var unit string
			if _, e := fmt.Sscanf(strings.TrimPrefix(s, "in "), "%d %s", &n, &unit); e == nil {
				switch {
				case strings.HasPrefix(unit, "min"):
					d = time.Duration(n) * time.Minute
				case strings.HasPrefix(unit, "hour"), strings.HasPrefix(unit, "hr"):
					d = time.Duration(n) * time.Hour
				case strings.HasPrefix(unit, "day"):
					d = time.Duration(n) * 24 * time.Hour
				default:
					return time.Time{}, fmt.Errorf("cannot understand %q", s)
				}
			} else {
				return time.Time{}, fmt.Errorf("cannot understand %q", s)
			}
		}
		return now.Add(d), nil
	}
	if rest == "" {
		rest = "09:00"
	}
	hhmm, err := normalizeTime(rest)
	if err != nil {
		return time.Time{}, fmt.Errorf("cannot understand time %q; use YYYY-MM-DD HH:MM, 'tomorrow 9am' or 'in 2 hours'", s)
	}
	hh, mm, _ := parseHHMM(hhmm)
	return time.Date(day.Year(), day.Month(), day.Day(), hh, mm, 0, 0, now.Location()), nil
}
