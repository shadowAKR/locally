// Package tools defines the local-machine operations the model may invoke, and
// classifies each invocation as safe (read-only) or risky (changes the system).
package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"localagent/internal/ollama"
)

// Risk describes how an invocation should be gated.
type Risk int

const (
	// Safe actions are read-only and run without confirmation.
	Safe Risk = iota
	// Risky actions change the system and require confirmation unless auto-approve is on.
	Risky
)

// Tool is one callable operation.
type Tool struct {
	Name        string
	Description string
	Parameters  map[string]any
	// Classify returns the risk of a specific invocation and a short reason.
	Classify func(args map[string]any) (Risk, string)
	// Run executes the tool. The returned string is shown to the model.
	Run func(ctx context.Context, args map[string]any) (string, error)
}

// Registry holds the available tools.
type Registry struct {
	tools map[string]*Tool
}

// NewRegistry returns a registry containing the default tool set.
func NewRegistry() *Registry {
	r := &Registry{tools: map[string]*Tool{}}
	for _, t := range defaultTools() {
		r.tools[t.Name] = t
	}
	for _, t := range browserTools() {
		r.tools[t.Name] = t
	}
	return r
}

// Add registers an extra tool (or replaces one with the same name).
func (r *Registry) Add(t *Tool) {
	r.tools[t.Name] = t
}

// Get looks up a tool by name.
func (r *Registry) Get(name string) (*Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// Names returns the tool names in stable order.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.tools))
	for n := range r.tools {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Definitions converts the registry into the format Ollama expects.
func (r *Registry) Definitions() []ollama.Tool {
	out := make([]ollama.Tool, 0, len(r.tools))
	for _, name := range r.Names() {
		t := r.tools[name]
		out = append(out, ollama.Tool{
			Type: "function",
			Function: ollama.ToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}
	return out
}

// Classify returns the risk for calling the named tool with args.
func (r *Registry) Classify(name string, args map[string]any) (Risk, string) {
	t, ok := r.tools[name]
	if !ok {
		return Risky, "unknown tool"
	}
	if t.Classify == nil {
		return Risky, "modifies the system"
	}
	return t.Classify(args)
}

// Run executes the named tool.
func (r *Registry) Run(ctx context.Context, name string, args map[string]any) (string, error) {
	t, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("unknown tool %q; available: %s", name, strings.Join(r.Names(), ", "))
	}
	if args == nil {
		args = map[string]any{}
	}
	return t.Run(ctx, args)
}

// ---- argument helpers ----

func strArg(args map[string]any, key string) string {
	v, ok := args[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprint(t)
	}
}

func boolArg(args map[string]any, key string) bool {
	v, ok := args[key]
	if !ok || v == nil {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(t, "true") || t == "1" || strings.EqualFold(t, "yes")
	default:
		return false
	}
}

func intArg(args map[string]any, key string, def int) int {
	v, ok := args[key]
	if !ok || v == nil {
		return def
	}
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case string:
		var n int
		if _, err := fmt.Sscanf(t, "%d", &n); err == nil {
			return n
		}
	}
	return def
}

func schema(props map[string]any, required ...string) map[string]any {
	s := map[string]any{
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func prop(typ, desc string) map[string]any {
	return map[string]any{"type": typ, "description": desc}
}

// truncate limits output size so a chatty command does not blow the context window.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + fmt.Sprintf("\n... [truncated %d bytes]", len(s)-max)
}
