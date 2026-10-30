package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

func defaultTools() []*Tool {
	return []*Tool{
		{
			Name: "run_shell",
			Description: "Run a bash command on the user's Linux laptop and return its combined stdout/stderr. " +
				"Use for anything without a dedicated tool: installing packages, managing processes, git, networking, settings. " +
				"Commands run non-interactively; never use commands that wait for input.",
			Parameters: schema(map[string]any{
				"command":         prop("string", "The bash command line to execute."),
				"cwd":             prop("string", "Optional working directory. Defaults to the home directory."),
				"timeout_seconds": prop("integer", "Optional timeout, default 60, max 600."),
			}, "command"),
			Classify: func(args map[string]any) (Risk, string) {
				return classifyShell(strArg(args, "command"))
			},
			Run: func(ctx context.Context, args map[string]any) (string, error) {
				cmd := strArg(args, "command")
				if strings.TrimSpace(cmd) == "" {
					return "", fmt.Errorf("command is required")
				}
				return runShell(ctx, cmd, strArg(args, "cwd"), intArg(args, "timeout_seconds", 60))
			},
		},
		{
			Name:        "read_file",
			Description: "Read a text file and return its contents. Large files are truncated.",
			Parameters: schema(map[string]any{
				"path": prop("string", "Path to the file. ~ is expanded; relative paths are relative to the home directory."),
			}, "path"),
			Classify: func(map[string]any) (Risk, string) { return Safe, "" },
			Run: func(ctx context.Context, args map[string]any) (string, error) {
				p := expandPath(strArg(args, "path"))
				if p == "" {
					return "", fmt.Errorf("path is required")
				}
				info, err := os.Stat(p)
				if err != nil {
					return "", err
				}
				if info.IsDir() {
					return "", fmt.Errorf("%s is a directory; use list_directory", p)
				}
				if info.Size() > 2*1024*1024 {
					return "", fmt.Errorf("file is %d bytes; too large to read whole. Use run_shell with head/tail/grep", info.Size())
				}
				data, err := os.ReadFile(p)
				if err != nil {
					return "", err
				}
				return truncate(string(data), 20000), nil
			},
		},
		{
			Name:        "write_file",
			Description: "Create or overwrite a text file with the given content. Parent directories are created. Set append=true to add to the end instead.",
			Parameters: schema(map[string]any{
				"path":    prop("string", "Destination path. ~ is expanded."),
				"content": prop("string", "Full text content to write."),
				"append":  prop("boolean", "If true, append instead of overwrite."),
			}, "path", "content"),
			Classify: func(args map[string]any) (Risk, string) {
				p := expandPath(strArg(args, "path"))
				if _, err := os.Stat(p); err == nil && !boolArg(args, "append") {
					return Risky, "overwrites an existing file"
				}
				return Risky, "writes a file"
			},
			Run: func(ctx context.Context, args map[string]any) (string, error) {
				p := expandPath(strArg(args, "path"))
				if p == "" {
					return "", fmt.Errorf("path is required")
				}
				if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
					return "", err
				}
				content := strArg(args, "content")
				flags := os.O_CREATE | os.O_WRONLY
				if boolArg(args, "append") {
					flags |= os.O_APPEND
				} else {
					flags |= os.O_TRUNC
				}
				f, err := os.OpenFile(p, flags, 0o644)
				if err != nil {
					return "", err
				}
				defer f.Close()
				if _, err := f.WriteString(content); err != nil {
					return "", err
				}
				return fmt.Sprintf("Wrote %d bytes to %s", len(content), p), nil
			},
		},
		{
			Name:        "list_directory",
			Description: "List the entries of a directory with sizes and modification times.",
			Parameters: schema(map[string]any{
				"path":        prop("string", "Directory path. Defaults to the home directory."),
				"show_hidden": prop("boolean", "Include dotfiles."),
			}),
			Classify: func(map[string]any) (Risk, string) { return Safe, "" },
			Run: func(ctx context.Context, args map[string]any) (string, error) {
				p := strArg(args, "path")
				if p == "" {
					p = "~"
				}
				p = expandPath(p)
				entries, err := os.ReadDir(p)
				if err != nil {
					return "", err
				}
				showHidden := boolArg(args, "show_hidden")
				var b strings.Builder
				fmt.Fprintf(&b, "%s (%d entries)\n", p, len(entries))
				sort.Slice(entries, func(i, j int) bool {
					if entries[i].IsDir() != entries[j].IsDir() {
						return entries[i].IsDir()
					}
					return entries[i].Name() < entries[j].Name()
				})
				n := 0
				for _, e := range entries {
					if !showHidden && strings.HasPrefix(e.Name(), ".") {
						continue
					}
					info, err := e.Info()
					if err != nil {
						continue
					}
					kind := "file"
					size := humanSize(info.Size())
					if e.IsDir() {
						kind = "dir "
						size = "-"
					}
					fmt.Fprintf(&b, "%s  %8s  %s  %s\n", kind, size, info.ModTime().Format("2006-01-02 15:04"), e.Name())
					n++
					if n >= 500 {
						b.WriteString("... (more entries omitted)\n")
						break
					}
				}
				return b.String(), nil
			},
		},
		{
			Name:        "find_files",
			Description: "Search for files by name pattern (glob like *.pdf or substring) under a directory. Returns matching paths.",
			Parameters: schema(map[string]any{
				"pattern":   prop("string", "Case-insensitive name pattern, e.g. '*.pdf' or 'report'."),
				"root":      prop("string", "Directory to search. Defaults to the home directory."),
				"max_depth": prop("integer", "Maximum directory depth, default 6."),
			}, "pattern"),
			Classify: func(map[string]any) (Risk, string) { return Safe, "" },
			Run: func(ctx context.Context, args map[string]any) (string, error) {
				pattern := strArg(args, "pattern")
				if pattern == "" {
					return "", fmt.Errorf("pattern is required")
				}
				if !strings.ContainsAny(pattern, "*?[") {
					pattern = "*" + pattern + "*"
				}
				root := strArg(args, "root")
				if root == "" {
					root = "~"
				}
				depth := intArg(args, "max_depth", 6)
				ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, "find", expandPath(root), "-maxdepth", fmt.Sprint(depth),
					"-not", "-path", "*/.*", "-not", "-path", "*/node_modules/*", "-iname", pattern)
				out, _ := cmd.Output()
				lines := strings.Split(strings.TrimSpace(string(out)), "\n")
				if len(lines) == 1 && lines[0] == "" {
					return "No files matched " + pattern, nil
				}
				if len(lines) > 200 {
					lines = append(lines[:200], fmt.Sprintf("... and %d more", len(lines)-200))
				}
				return strings.Join(lines, "\n"), nil
			},
		},
		{
			Name:        "system_info",
			Description: "Get a labelled snapshot of the laptop: OS, uptime, CPU load, memory, disk usage, battery, network and top processes. Use this for any health/status question.",
			Parameters:  schema(map[string]any{}),
			Classify:    func(map[string]any) (Risk, string) { return Safe, "" },
			Run: func(ctx context.Context, args map[string]any) (string, error) {
				return systemInfo(ctx), nil
			},
		},
		{
			Name: "folder_sizes",
			Description: "Measure how much disk space each item inside a folder takes, largest first. Use this to find what is using disk space; " +
				"start at the home folder (or '/') and then call it again on the biggest sub-folder to drill down.",
			Parameters: schema(map[string]any{
				"path": prop("string", "Folder to measure. Defaults to the home directory."),
				"top":  prop("integer", "How many of the largest entries to show, default 15."),
			}),
			Classify: func(map[string]any) (Risk, string) { return Safe, "" },
			Run: func(ctx context.Context, args map[string]any) (string, error) {
				p := strArg(args, "path")
				if p == "" {
					p = "~"
				}
				return folderSizes(ctx, expandPath(p), intArg(args, "top", 15))
			},
		},
		{
			Name: "open",
			Description: "Launch a desktop application by name (e.g. 'firefox', 'org.gnome.Calculator', 'code'), or open a file or folder with its default app. " +
				"Do NOT use this to read websites or emails; use browser_open for that, which returns the page content.",
			Parameters: schema(map[string]any{
				"target": prop("string", "A file path, folder, URL (https://...), or application name."),
			}, "target"),
			Classify: func(map[string]any) (Risk, string) { return Safe, "" },
			Run: func(ctx context.Context, args map[string]any) (string, error) {
				target := strings.TrimSpace(strArg(args, "target"))
				if target == "" {
					return "", fmt.Errorf("target is required")
				}
				isURL := strings.Contains(target, "://") || strings.HasPrefix(target, "www.")
				if isURL {
					if strings.HasPrefix(target, "www.") {
						target = "https://" + target
					}
					return launchDetached("xdg-open", target)
				}
				p := expandPath(target)
				if _, err := os.Stat(p); err == nil {
					return launchDetached("xdg-open", p)
				}
				// Treat as application name.
				if _, err := exec.LookPath(target); err == nil {
					return launchDetached(target)
				}
				if _, err := exec.LookPath("gtk-launch"); err == nil {
					if id := findDesktopID(target); id != "" {
						return launchDetached("gtk-launch", id)
					}
				}
				return "", fmt.Errorf("could not find a file, URL or application named %q", target)
			},
		},
		{
			Name:        "notify",
			Description: "Show a desktop notification to the user.",
			Parameters: schema(map[string]any{
				"title":   prop("string", "Notification title."),
				"message": prop("string", "Notification body text."),
			}, "title"),
			Classify: func(map[string]any) (Risk, string) { return Safe, "" },
			Run: func(ctx context.Context, args map[string]any) (string, error) {
				title := strArg(args, "title")
				if title == "" {
					title = "Local Agent"
				}
				cmd := exec.CommandContext(ctx, "notify-send", title, strArg(args, "message"))
				if out, err := cmd.CombinedOutput(); err != nil {
					return "", fmt.Errorf("notify-send failed: %v %s", err, out)
				}
				return "Notification shown", nil
			},
		},
		{
			Name:        "trash",
			Description: "Move a file or folder to the trash (recoverable). Prefer this over 'rm'.",
			Parameters: schema(map[string]any{
				"path": prop("string", "Path to move to trash."),
			}, "path"),
			Classify: func(args map[string]any) (Risk, string) {
				return Risky, "moves " + strArg(args, "path") + " to trash"
			},
			Run: func(ctx context.Context, args map[string]any) (string, error) {
				p := expandPath(strArg(args, "path"))
				if p == "" {
					return "", fmt.Errorf("path is required")
				}
				if _, err := os.Stat(p); err != nil {
					return "", err
				}
				cmd := exec.CommandContext(ctx, "gio", "trash", p)
				if out, err := cmd.CombinedOutput(); err != nil {
					return "", fmt.Errorf("gio trash failed: %v %s", err, strings.TrimSpace(string(out)))
				}
				return "Moved to trash: " + p, nil
			},
		},
	}
}

// launchDetached starts a program in its own session so it outlives the agent.
func launchDetached(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to launch %s: %w", name, err)
	}
	go cmd.Wait() //nolint:errcheck
	return fmt.Sprintf("Launched: %s %s", name, strings.Join(args, " ")), nil
}

// findDesktopID finds a .desktop file whose id or Name matches the query.
func findDesktopID(query string) string {
	q := strings.ToLower(query)
	dirs := []string{"/usr/share/applications", "/usr/local/share/applications",
		filepath.Join(os.Getenv("HOME"), ".local/share/applications"),
		"/var/lib/flatpak/exports/share/applications", "/var/lib/snapd/desktop/applications"}
	var fallback string
	for _, d := range dirs {
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".desktop") {
				continue
			}
			id := strings.TrimSuffix(e.Name(), ".desktop")
			lid := strings.ToLower(id)
			if lid == q || strings.HasSuffix(lid, "."+q) {
				return id
			}
			if fallback == "" && strings.Contains(lid, q) {
				fallback = id
				continue
			}
			if fallback == "" {
				if data, err := os.ReadFile(filepath.Join(d, e.Name())); err == nil {
					for _, line := range strings.Split(string(data), "\n") {
						if strings.HasPrefix(line, "Name=") && strings.EqualFold(strings.TrimPrefix(line, "Name="), query) {
							fallback = id
							break
						}
					}
				}
			}
		}
	}
	return fallback
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
