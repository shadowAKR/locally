// Command agentcli is a terminal harness for the agent, useful for testing the
// backend without the desktop UI. Usage: go run ./cmd/agentcli [-model name] [-yes] "message"
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"localagent/internal/agent"
	"localagent/internal/config"
	"localagent/internal/ollama"
	"localagent/internal/tools"
)

type stdinApprover struct{}

func (stdinApprover) Ask(ctx context.Context, id, tool string, args map[string]any, reason string) (bool, error) {
	b, _ := json.MarshalIndent(args, "", "  ")
	fmt.Fprintf(os.Stderr, "\n⚠  %s wants to run %s (%s):\n%s\nApprove? [y/N] ", "agent", tool, reason, b)
	r := bufio.NewReader(os.Stdin)
	line, _ := r.ReadString('\n')
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "y" || line == "yes", nil
}

func main() {
	settings := config.Load()
	model := flag.String("model", settings.Model, "ollama model name")
	yes := flag.Bool("yes", false, "auto-approve risky actions")
	think := flag.Bool("think", settings.Think, "enable model thinking")
	verbose := flag.Bool("v", false, "print tool output")
	flag.Parse()

	client := ollama.New(settings.OllamaURL)
	if *model == "" {
		models, err := client.ListModels(context.Background())
		if err != nil || len(models) == 0 {
			fmt.Fprintln(os.Stderr, "no model available:", err)
			os.Exit(1)
		}
		*model = models[0].Name
	}

	emit := func(ev agent.Event) {
		switch ev.Type {
		case "delta":
			fmt.Print(ev.Text)
		case "thinking":
			if *verbose {
				fmt.Fprint(os.Stderr, "\033[2m"+ev.Text+"\033[0m")
			}
		case "tool_call":
			flag := ""
			if ev.Risky {
				flag = " [risky: " + ev.Reason + "]"
			}
			fmt.Fprintf(os.Stderr, "\n\033[36m→ %s %s%s\033[0m\n", ev.Tool, ev.Args, flag)
		case "tool_result":
			if *verbose || ev.IsError {
				fmt.Fprintf(os.Stderr, "\033[2m%s\033[0m\n", strings.TrimSpace(ev.Output))
			} else {
				fmt.Fprintf(os.Stderr, "\033[2m  (%d bytes in %dms)\033[0m\n", len(ev.Output), ev.Duration)
			}
		case "error":
			fmt.Fprintf(os.Stderr, "\n\033[31merror: %s\033[0m\n", ev.Text)
		case "done":
			fmt.Println()
		}
	}

	ag := agent.New(client, tools.NewRegistry(), stdinApprover{}, agent.Config{
		Model: *model, Think: *think, AutoApprove: *yes, NumCtx: settings.NumCtx, Temperature: settings.Temperature,
	}, emit)

	fmt.Fprintf(os.Stderr, "model: %s  auto-approve: %v\n", *model, *yes)
	if msg := strings.Join(flag.Args(), " "); msg != "" {
		if err := ag.Run(context.Background(), msg); err != nil {
			os.Exit(1)
		}
		return
	}
	// Interactive REPL.
	r := bufio.NewReader(os.Stdin)
	for {
		fmt.Fprint(os.Stderr, "\n> ")
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line == "/quit" || line == "/exit" {
			return
		}
		_ = ag.Run(context.Background(), line)
	}
}
