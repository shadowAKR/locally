package tools

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestSystemInfoLabelled(t *testing.T) {
	out := systemInfo(context.Background())
	for _, want := range []string{"HOST:", "MEMORY:", "DISK /:", "CPU LOAD:"} {
		if !strings.Contains(out, want) {
			t.Errorf("system_info missing %q:\n%s", want, out)
		}
	}
	t.Log("\n" + out)
}

func TestFolderSizes(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(dir+"/big.bin", make([]byte, 300*1024), 0o644)
	os.MkdirAll(dir+"/sub", 0o755)
	os.WriteFile(dir+"/sub/small.txt", []byte("x"), 0o644)
	out, err := folderSizes(context.Background(), dir, 10)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 3 || !strings.Contains(lines[1], "big.bin") || !strings.Contains(lines[2], "sub/") {
		t.Fatalf("unexpected ordering:\n%s", out)
	}
	t.Log("\n" + out)
}
