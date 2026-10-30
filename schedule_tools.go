package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"localagent/internal/schedule"
	"localagent/internal/tools"
)

// scheduleTools lets the model create and manage scheduled tasks from chat.
func scheduleTools(s *schedule.Scheduler) []*tools.Tool {
	str := func(args map[string]any, k string) string {
		if v, ok := args[k]; ok && v != nil {
			return strings.TrimSpace(fmt.Sprint(v))
		}
		return ""
	}
	return []*tools.Tool{
		{
			Name: "schedule_task",
			Description: "Schedule an instruction to be run automatically later by this assistant. Use when the user says 'every day at...', 'at 9am', 'tomorrow', 'when the laptop starts', 'remind me at...'. " +
				"kind=daily needs time (HH:MM 24h, e.g. 18:30); kind=once needs when (e.g. '2026-10-31 09:00', 'tomorrow 9am', 'in 2 hours'); kind=startup runs each time the app starts. " +
				"Write the prompt as a complete instruction to yourself, e.g. 'Open Gmail and summarize unread emails, then notify me'.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"prompt": map[string]any{"type": "string", "description": "The instruction to run later."},
					"kind":   map[string]any{"type": "string", "enum": []string{"startup", "daily", "once"}, "description": "When to run."},
					"time":   map[string]any{"type": "string", "description": "For daily: HH:MM in 24-hour time."},
					"when":   map[string]any{"type": "string", "description": "For once: date and time, e.g. '2026-10-31 09:00' or 'tomorrow 9am'."},
					"name":   map[string]any{"type": "string", "description": "Short label for the task."},
				},
				"required": []string{"prompt", "kind"},
			},
			Classify: func(args map[string]any) (tools.Risk, string) {
				return tools.Risky, "creates a scheduled task (" + str(args, "kind") + ")"
			},
			Run: func(ctx context.Context, args map[string]any) (string, error) {
				kind := schedule.Kind(strings.ToLower(str(args, "kind")))
				var at time.Time
				if kind == schedule.Once {
					var err error
					if at, err = schedule.ParseWhen(str(args, "when"), time.Now()); err != nil {
						return "", err
					}
				}
				t, err := s.Add(str(args, "name"), str(args, "prompt"), kind, str(args, "time"), at)
				if err != nil {
					return "", err
				}
				return "Scheduled task created: " + describeTask(t), nil
			},
		},
		{
			Name:        "list_scheduled_tasks",
			Description: "List the scheduled tasks and when they run next.",
			Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
			Classify:    func(map[string]any) (tools.Risk, string) { return tools.Safe, "" },
			Run: func(ctx context.Context, args map[string]any) (string, error) {
				list := s.List()
				if len(list) == 0 {
					return "No scheduled tasks.", nil
				}
				var b strings.Builder
				for _, t := range list {
					b.WriteString("- " + describeTask(t) + "\n")
				}
				return b.String(), nil
			},
		},
		{
			Name:        "delete_scheduled_task",
			Description: "Delete a scheduled task by its id or name.",
			Parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{"task": map[string]any{"type": "string", "description": "Task id or (part of) its name."}},
				"required":   []string{"task"},
			},
			Classify: func(args map[string]any) (tools.Risk, string) {
				return tools.Risky, "deletes scheduled task " + str(args, "task")
			},
			Run: func(ctx context.Context, args map[string]any) (string, error) {
				q := strings.ToLower(str(args, "task"))
				for _, t := range s.List() {
					if t.ID == q || strings.Contains(strings.ToLower(t.Name), q) {
						if err := s.Delete(t.ID); err != nil {
							return "", err
						}
						return "Deleted scheduled task: " + t.Name, nil
					}
				}
				return "", fmt.Errorf("no scheduled task matches %q", q)
			},
		},
	}
}

func describeTask(t schedule.Task) string {
	when := ""
	switch t.Kind {
	case schedule.Startup:
		when = "every time the app starts"
	case schedule.Daily:
		when = "every day at " + t.Time
	case schedule.Once:
		when = "once at " + t.At.Format("Mon 2 Jan 15:04")
	}
	state := "enabled"
	if !t.Enabled {
		state = "disabled"
	}
	next := ""
	if !t.NextRun.IsZero() {
		next = ", next run " + t.NextRun.Format("Mon 2 Jan 15:04")
	}
	return fmt.Sprintf("%s (id %s): %s, %s%s. Prompt: %q", t.Name, t.ID, when, state, next, t.Prompt)
}
