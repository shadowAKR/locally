package schedule

import (
	"path/filepath"
	"testing"
	"time"
)

func TestAddListNext(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "s.json"), func(Task) (string, error) { return "", nil })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add("", "say hi", Daily, "9pm", time.Time{}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add("", "boot check", Startup, "", time.Time{}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add("", "late", Once, "", time.Now().Add(-time.Hour)); err == nil {
		t.Fatal("past one-off should be rejected")
	}
	if _, err := s.Add("", "bad", Daily, "25:00", time.Time{}); err == nil {
		t.Fatal("bad time should be rejected")
	}
	list := s.List()
	if len(list) != 2 {
		t.Fatalf("want 2 tasks, got %d", len(list))
	}
	var daily Task
	for _, x := range list {
		if x.Kind == Daily {
			daily = x
		}
	}
	if daily.Time != "21:00" || daily.NextRun.IsZero() || daily.NextRun.Hour() != 21 {
		t.Fatalf("daily next run wrong: %+v", daily)
	}
	// Reload from disk.
	s2, err := Load(s.path, nil)
	if err != nil || len(s2.List()) != 2 {
		t.Fatalf("reload failed: %v", err)
	}
}

func TestParseWhen(t *testing.T) {
	now := time.Date(2026, 10, 30, 18, 0, 0, 0, time.Local)
	cases := map[string]time.Time{
		"2026-10-31 09:00": time.Date(2026, 10, 31, 9, 0, 0, 0, time.Local),
		"tomorrow 9am":     time.Date(2026, 10, 31, 9, 0, 0, 0, time.Local),
		"today 18:30":      time.Date(2026, 10, 30, 18, 30, 0, 0, time.Local),
		"in 2 hours":       now.Add(2 * time.Hour),
		"in 30 minutes":    now.Add(30 * time.Minute),
	}
	for in, want := range cases {
		got, err := ParseWhen(in, now)
		if err != nil || !got.Equal(want) {
			t.Errorf("ParseWhen(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	if _, err := ParseWhen("whenever", now); err == nil {
		t.Error("expected error for nonsense")
	}
}
