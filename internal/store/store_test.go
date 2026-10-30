package store

import (
	"testing"

	"localagent/internal/ollama"
)

func TestRoundTrip(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c := NewChat("user")
	if err := s.Save(c); err != nil {
		t.Fatal(err)
	}
	if l, _ := s.List(); len(l) != 0 {
		t.Fatal("empty chat should not be saved")
	}
	c.Messages = append(c.Messages, ollama.Message{Role: "user", Content: "summarize my emails please"}, ollama.Message{Role: "assistant", Content: "done"})
	if err := s.Save(c); err != nil {
		t.Fatal(err)
	}
	l, _ := s.List()
	if len(l) != 1 || l[0].Title != "summarize my emails please" || l[0].Count != 2 {
		t.Fatalf("bad listing: %+v", l)
	}
	got, err := s.Load(c.ID)
	if err != nil || len(got.Messages) != 2 {
		t.Fatalf("load: %v %+v", err, got)
	}
	if err := s.Delete(c.ID); err != nil {
		t.Fatal(err)
	}
	if l, _ := s.List(); len(l) != 0 {
		t.Fatal("delete failed")
	}
}
