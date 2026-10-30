package tools

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileTools(t *testing.T) {
	reg := NewRegistry()
	ctx := context.Background()
	dir := t.TempDir()
	p := filepath.Join(dir, "notes", "hello.txt")

	if risk, _ := reg.Classify("write_file", map[string]any{"path": p, "content": "x"}); risk != Risky {
		t.Fatal("write_file should be risky")
	}
	out, err := reg.Run(ctx, "write_file", map[string]any{"path": p, "content": "hello from the agent\n"})
	if err != nil || !strings.Contains(out, "Wrote") {
		t.Fatalf("write_file: %v %q", err, out)
	}
	out, err = reg.Run(ctx, "write_file", map[string]any{"path": p, "content": "second line\n", "append": true})
	if err != nil {
		t.Fatalf("append: %v %q", err, out)
	}
	out, err = reg.Run(ctx, "read_file", map[string]any{"path": p})
	if err != nil || out != "hello from the agent\nsecond line\n" {
		t.Fatalf("read_file: %v %q", err, out)
	}
	out, err = reg.Run(ctx, "list_directory", map[string]any{"path": filepath.Join(dir, "notes")})
	if err != nil || !strings.Contains(out, "hello.txt") {
		t.Fatalf("list_directory: %v %q", err, out)
	}
	out, err = reg.Run(ctx, "find_files", map[string]any{"pattern": "hello", "root": dir})
	if err != nil || !strings.Contains(out, p) {
		t.Fatalf("find_files: %v %q", err, out)
	}
	out, err = reg.Run(ctx, "run_shell", map[string]any{"command": "wc -l " + p})
	if err != nil || !strings.HasPrefix(strings.TrimSpace(out), "2") {
		t.Fatalf("run_shell: %v %q", err, out)
	}
	out, err = reg.Run(ctx, "run_shell", map[string]any{"command": "exit 3"})
	if err != nil || !strings.Contains(out, "exit code 3") {
		t.Fatalf("run_shell exit code: %v %q", err, out)
	}
	_, err = reg.Run(ctx, "run_shell", map[string]any{"command": "sleep 5", "timeout_seconds": 1})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout, got %v", err)
	}
	if _, err := reg.Run(ctx, "read_file", map[string]any{"path": dir}); err == nil {
		t.Fatal("reading a directory should fail")
	}
	if len(reg.Definitions()) != len(reg.Names()) {
		t.Fatal("definitions/names mismatch")
	}
}
