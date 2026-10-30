// Package store persists chat conversations as JSON files on disk.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"localagent/internal/ollama"
)

// Chat is one saved conversation.
type Chat struct {
	ID        string           `json:"id"`
	Title     string           `json:"title"`
	Source    string           `json:"source"` // "user" or "schedule"
	CreatedAt time.Time        `json:"createdAt"`
	UpdatedAt time.Time        `json:"updatedAt"`
	Messages  []ollama.Message `json:"messages"`
}

// Summary is the lightweight listing entry shown in the sidebar.
type Summary struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Source    string    `json:"source"`
	UpdatedAt time.Time `json:"updatedAt"`
	Count     int       `json:"count"`
}

// Store writes chats to a directory, one file per chat.
type Store struct {
	dir string
}

// Dir returns the default chat directory (~/.local/share/localagent/chats).
func Dir() string {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "localagent", "chats")
}

// Open returns a store rooted at dir, creating it if needed.
func Open(dir string) (*Store, error) {
	if dir == "" {
		dir = Dir()
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

// Path returns the directory the store writes to.
func (s *Store) Path() string { return s.dir }

// NewChat creates an empty chat with a fresh id.
func NewChat(source string) *Chat {
	now := time.Now()
	return &Chat{
		ID:        now.Format("20060102-150405") + "-" + fmt.Sprintf("%04d", now.Nanosecond()/100000),
		Source:    source,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// TitleFrom derives a short title from the first user message.
func TitleFrom(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > 60 {
		text = text[:57] + "..."
	}
	if text == "" {
		return "New chat"
	}
	return text
}

func (s *Store) file(id string) string {
	return filepath.Join(s.dir, filepath.Base(id)+".json")
}

// Save writes the chat to disk. Empty chats are not persisted.
func (s *Store) Save(c *Chat) error {
	if c == nil || len(c.Messages) == 0 {
		return nil
	}
	c.UpdatedAt = time.Now()
	if c.Title == "" {
		for _, m := range c.Messages {
			if m.Role == "user" {
				c.Title = TitleFrom(m.Content)
				break
			}
		}
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.file(c.ID) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.file(c.ID))
}

// Load reads one chat.
func (s *Store) Load(id string) (*Chat, error) {
	data, err := os.ReadFile(s.file(id))
	if err != nil {
		return nil, err
	}
	var c Chat
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Delete removes one chat.
func (s *Store) Delete(id string) error {
	err := os.Remove(s.file(id))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// List returns all chats, newest first.
func (s *Store) List() ([]Summary, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	var out []Summary
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		c, err := s.Load(strings.TrimSuffix(e.Name(), ".json"))
		if err != nil {
			continue
		}
		out = append(out, Summary{ID: c.ID, Title: c.Title, Source: c.Source, UpdatedAt: c.UpdatedAt, Count: len(c.Messages)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}
