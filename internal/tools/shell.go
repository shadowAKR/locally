package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

// readOnlyCommands run without confirmation when used without redirection.
var readOnlyCommands = map[string]bool{
	"ls": true, "cat": true, "head": true, "tail": true, "pwd": true, "echo": true,
	"grep": true, "egrep": true, "fgrep": true, "rg": true, "find": true, "fd": true,
	"wc": true, "which": true, "whereis": true, "type": true, "uname": true, "df": true,
	"du": true, "free": true, "uptime": true, "ps": true, "pgrep": true, "whoami": true,
	"id": true, "date": true, "cal": true, "hostname": true, "hostnamectl": true,
	"lscpu": true, "lsblk": true, "lsusb": true, "lspci": true, "lsmem": true, "ip": true,
	"ss": true, "nproc": true, "env": true, "printenv": true, "file": true, "stat": true,
	"tree": true, "sort": true, "uniq": true, "cut": true, "awk": true, "tr": true,
	"column": true, "jq": true, "basename": true, "dirname": true, "realpath": true,
	"readlink": true, "md5sum": true, "sha256sum": true, "nmcli": false, "ping": true,
	"dig": true, "host": true, "nslookup": true, "curl": true, "wget": false, "upower": true,
	"acpi": true, "sensors": true, "lsof": true, "getent": true, "locale": true,
	"timedatectl": true, "journalctl": true, "dmesg": true, "mount": true, "lsmod": true,
	"xdg-user-dir": true, "gsettings": false, "true": true, "false": true, "test": true,
	"seq": true, "printf": true, "expr": true, "bc": true, "sleep": true, "xrandr": true,
	"pactl": false, "amixer": false, "brightnessctl": false, "apt-cache": true, "dpkg-query": true,
	"snap": false, "flatpak": false, "pip": false, "npm": false, "go": false,
}

// readOnlySubcommands allows specific sub-commands of otherwise risky tools.
var readOnlySubcommands = map[string]map[string]bool{
	"git":           {"status": true, "log": true, "diff": true, "show": true, "branch": true, "remote": true, "blame": true, "ls-files": true},
	"systemctl":     {"status": true, "list-units": true, "is-active": true, "is-enabled": true, "show": true, "list-timers": true, "cat": true},
	"apt":           {"list": true, "show": true, "search": true, "policy": true},
	"dpkg":          {"-l": true, "-L": true, "-s": true, "--list": true},
	"nmcli":         {"device": true, "connection": true, "general": true, "radio": true, "networking": true},
	"gsettings":     {"get": true, "list-keys": true, "list-schemas": true},
	"pactl":         {"list": true, "info": true, "get-sink-volume": true, "get-sink-mute": true, "get-default-sink": true},
	"docker":        {"ps": true, "images": true, "logs": true, "inspect": true, "stats": true},
	"snap":          {"list": true, "info": true, "find": true},
	"flatpak":       {"list": true, "info": true, "search": true},
	"pip":           {"list": true, "show": true, "freeze": true},
	"npm":           {"ls": true, "list": true, "view": true, "outdated": true},
	"go":            {"version": true, "env": true, "list": true},
	"brightnessctl": {"get": true, "max": true, "info": true},
}

// Sub-commands of allowed tools that still mutate state.
var mutatingSubcommands = map[string]map[string]bool{
	"nmcli": {"up": true, "down": true, "add": true, "modify": true, "delete": true, "off": true, "on": true, "connect": true, "disconnect": true},
}

var redirectRe = regexp.MustCompile(`(^|[^<])>|\btee\b`)
var segmentSplitRe = regexp.MustCompile(`\|\|?|&&|;|\n`)

// classifyShell decides whether a shell command line is read-only.
func classifyShell(command string) (Risk, string) {
	cmd := strings.TrimSpace(command)
	if cmd == "" {
		return Safe, ""
	}
	if redirectRe.MatchString(cmd) {
		return Risky, "writes output to a file"
	}
	if strings.Contains(cmd, "$(") || strings.Contains(cmd, "`") {
		return Risky, "uses command substitution"
	}
	for _, seg := range segmentSplitRe.Split(cmd, -1) {
		fields := strings.Fields(seg)
		// Skip leading VAR=value assignments.
		for len(fields) > 0 && strings.Contains(fields[0], "=") && !strings.HasPrefix(fields[0], "-") {
			fields = fields[1:]
		}
		if len(fields) == 0 {
			continue
		}
		name := filepath.Base(fields[0])
		switch name {
		case "sudo", "su", "doas", "pkexec":
			return Risky, "requires elevated privileges"
		case "rm", "rmdir", "mv", "cp", "dd", "mkfs", "shred", "chmod", "chown", "kill", "killall", "pkill", "shutdown", "reboot", "poweroff", "halt":
			return Risky, "destructive command: " + name
		case "sed":
			for _, f := range fields[1:] {
				if f == "-i" || strings.HasPrefix(f, "-i") || strings.HasPrefix(f, "--in-place") {
					return Risky, "sed edits a file in place"
				}
			}
			continue
		case "xargs", "sh", "bash", "zsh", "eval", "exec", "python", "python3", "perl", "node":
			return Risky, "runs arbitrary code via " + name
		}
		if allowed, known := readOnlyCommands[name]; known && allowed {
			continue
		}
		if subs, ok := readOnlySubcommands[name]; ok {
			if len(fields) > 1 {
				if mut, ok := mutatingSubcommands[name]; ok {
					for _, f := range fields[1:] {
						if mut[f] {
							return Risky, name + " " + f + " changes system state"
						}
					}
				}
				if subs[fields[1]] {
					continue
				}
			}
			return Risky, name + " may change system state"
		}
		return Risky, "command '" + name + "' is not on the read-only list"
	}
	return Safe, ""
}

// expandPath resolves ~ and makes relative paths absolute from the user's home.
func expandPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return p
	}
	home, _ := os.UserHomeDir()
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:])
	}
	if !filepath.IsAbs(p) {
		return filepath.Join(home, p)
	}
	return filepath.Clean(p)
}

// runShell executes a command line with bash, with a timeout and output cap.
func runShell(ctx context.Context, command, cwd string, timeoutSec int) (string, error) {
	if timeoutSec <= 0 || timeoutSec > 600 {
		timeoutSec = 60
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-lc", command)
	if cwd != "" {
		cmd.Dir = expandPath(cwd)
	} else {
		cmd.Dir, _ = os.UserHomeDir()
	}
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// Kill the whole process group on timeout so pipelines don't linger.
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = 2 * time.Second

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()

	result := truncate(out.String(), 12000)
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return result, fmt.Errorf("command timed out after %ds", timeoutSec)
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if strings.TrimSpace(result) == "" {
				result = "(no output)"
			}
			return result + fmt.Sprintf("\n[exit code %d]", exitErr.ExitCode()), nil
		}
		return result, err
	}
	if strings.TrimSpace(result) == "" {
		return "(command succeeded with no output)", nil
	}
	return result, nil
}
