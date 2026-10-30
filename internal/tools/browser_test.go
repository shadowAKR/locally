package tools

import (
	"context"
	"os"
	"strings"
	"testing"
)

// Run with LOCALAGENT_BROWSER_TEST=1; it opens a real Chrome window.
func TestBrowserOpenSmoke(t *testing.T) {
	if os.Getenv("LOCALAGENT_BROWSER_TEST") == "" {
		t.Skip("set LOCALAGENT_BROWSER_TEST=1 to run")
	}
	t.Cleanup(CloseBrowser)
	reg := NewRegistry()
	out, err := reg.Run(context.Background(), "browser_open", map[string]any{"url": "https://example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Example Domain") {
		t.Fatalf("unexpected page text:\n%s", out)
	}
	t.Log("\n" + out)
	out, err = reg.Run(context.Background(), "browser_click", map[string]any{"text": "Learn more"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "iana.org") {
		t.Fatalf("click did not navigate:\n%s", truncate(out, 600))
	}
	t.Log("\n" + truncate(out, 400))
}
