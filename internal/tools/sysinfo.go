package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// systemInfo builds a labelled, unambiguous snapshot of the machine. Every
// value is spelled out ("16 GB free of 94 GB") so small models cannot confuse
// table columns.
func systemInfo(ctx context.Context) string {
	var b strings.Builder
	w := func(format string, a ...any) { fmt.Fprintf(&b, format+"\n", a...) }

	// Host
	host, _ := os.Hostname()
	osName := "Linux"
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				osName = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), `"`)
			}
		}
	}
	username := ""
	if u, err := user.Current(); err == nil {
		username = u.Username
	}
	model := strings.TrimSpace(readFile("/sys/devices/virtual/dmi/id/product_name"))
	vendor := strings.TrimSpace(readFile("/sys/devices/virtual/dmi/id/sys_vendor"))
	w("HOST: %s, user %s, %s", host, username, osName)
	if model != "" {
		w("HARDWARE: %s %s", vendor, model)
	}

	// Uptime & load
	if up := readFile("/proc/uptime"); up != "" {
		if secs, err := strconv.ParseFloat(strings.Fields(up)[0], 64); err == nil {
			d := time.Duration(secs) * time.Second
			w("UPTIME: %s", humanDuration(d))
		}
	}
	if la := strings.Fields(readFile("/proc/loadavg")); len(la) >= 3 {
		cores := cpuCount()
		w("CPU LOAD: 1-minute average %s, 5-minute %s, 15-minute %s (machine has %d logical CPUs; load above %d means fully busy)", la[0], la[1], la[2], cores, cores)
	}
	if name := cpuModel(); name != "" {
		w("CPU MODEL: %s", name)
	}

	// Memory
	mem := parseMeminfo()
	if total, ok := mem["MemTotal"]; ok {
		avail := mem["MemAvailable"]
		used := total - avail
		pct := 0.0
		if total > 0 {
			pct = float64(used) / float64(total) * 100
		}
		w("MEMORY: %s used of %s total (%.0f%% used), %s available", humanKB(used), humanKB(total), pct, humanKB(avail))
		if st, ok := mem["SwapTotal"]; ok && st > 0 {
			w("SWAP: %s used of %s", humanKB(st-mem["SwapFree"]), humanKB(st))
		}
	}

	// Disks
	seen := map[string]bool{}
	for _, mnt := range []string{"/", "/home"} {
		var st syscall.Statfs_t
		if err := syscall.Statfs(mnt, &st); err != nil {
			continue
		}
		key := fmt.Sprintf("%d", st.Fsid)
		if seen[key] {
			continue
		}
		seen[key] = true
		total := st.Blocks * uint64(st.Bsize)
		free := st.Bavail * uint64(st.Bsize)
		used := total - free
		pct := 0.0
		if total > 0 {
			pct = float64(used) / float64(total) * 100
		}
		w("DISK %s: %s used of %s total (%.0f%% used), %s free", mnt, humanSize(int64(used)), humanSize(int64(total)), pct, humanSize(int64(free)))
	}

	// Battery
	bats, _ := filepath.Glob("/sys/class/power_supply/BAT*")
	if len(bats) == 0 {
		w("BATTERY: none (desktop or no battery detected)")
	}
	for _, bdir := range bats {
		cap := strings.TrimSpace(readFile(filepath.Join(bdir, "capacity")))
		status := strings.TrimSpace(readFile(filepath.Join(bdir, "status")))
		w("BATTERY: %s%%, %s", cap, strings.ToLower(status))
	}

	// Network via nmcli (clear device states), fallback to ip.
	netLines := networkSummary(ctx)
	if len(netLines) == 0 {
		netLines = []string{"no active network connection detected"}
	}
	for _, l := range netLines {
		w("NETWORK: %s", l)
	}

	// Top processes
	if top := topProcesses(ctx); top != "" {
		w("TOP PROCESSES BY MEMORY:\n%s", top)
	}
	return strings.TrimRight(b.String(), "\n")
}

func readFile(p string) string {
	data, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return string(data)
}

func cpuCount() int {
	n := 0
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if strings.HasPrefix(sc.Text(), "processor") {
			n++
		}
	}
	return n
}

func cpuModel() string {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if strings.HasPrefix(sc.Text(), "model name") {
			parts := strings.SplitN(sc.Text(), ":", 2)
			if len(parts) == 2 {
				return strings.Join(strings.Fields(parts[1]), " ")
			}
		}
	}
	return ""
}

func parseMeminfo() map[string]int64 {
	out := map[string]int64{}
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		if v, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
			out[key] = v
		}
	}
	return out
}

func networkSummary(ctx context.Context) []string {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var lines []string
	out, err := exec.CommandContext(ctx, "nmcli", "-t", "-f", "DEVICE,TYPE,STATE,CONNECTION", "device").Output()
	if err == nil {
		for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			p := strings.Split(l, ":")
			if len(p) < 4 || p[1] == "loopback" || p[1] == "wifi-p2p" || p[1] == "bridge" || strings.HasPrefix(p[0], "docker") || strings.HasPrefix(p[0], "br-") || strings.HasPrefix(p[0], "veth") {
				continue
			}
			kind := p[1]
			switch kind {
			case "wifi":
				kind = "Wi-Fi"
			case "ethernet":
				kind = "Ethernet"
			}
			if strings.HasPrefix(p[2], "connected") {
				lines = append(lines, fmt.Sprintf("%s (%s) is connected to \"%s\"", kind, p[0], p[3]))
			} else {
				lines = append(lines, fmt.Sprintf("%s (%s) is %s", kind, p[0], p[2]))
			}
		}
		return lines
	}
	out, err = exec.CommandContext(ctx, "ip", "-brief", "addr").Output()
	if err != nil {
		return nil
	}
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.Fields(l)
		if len(f) < 2 || f[0] == "lo" {
			continue
		}
		if len(f) >= 3 && f[1] == "UP" {
			lines = append(lines, fmt.Sprintf("%s is up with address %s", f[0], f[2]))
		} else {
			lines = append(lines, fmt.Sprintf("%s is %s", f[0], strings.ToLower(f[1])))
		}
	}
	return lines
}

func topProcesses(ctx context.Context) string {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ps", "-eo", "rss,pcpu,comm", "--sort=-rss", "--no-headers").Output()
	if err != nil {
		return ""
	}
	var lines []string
	for i, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if i >= 5 {
			break
		}
		f := strings.Fields(l)
		if len(f) < 3 {
			continue
		}
		rss, _ := strconv.ParseInt(f[0], 10, 64)
		lines = append(lines, fmt.Sprintf("  %s uses %s memory, %s%% CPU", f[2], humanKB(rss), f[1]))
	}
	return strings.Join(lines, "\n")
}

func humanKB(kb int64) string { return humanSize(kb * 1024) }

func humanDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	switch {
	case days > 0:
		return fmt.Sprintf("%d days %d hours", days, hours)
	case hours > 0:
		return fmt.Sprintf("%d hours %d minutes", hours, mins)
	default:
		return fmt.Sprintf("%d minutes", mins)
	}
}

// folderSizes reports the largest entries directly under dir.
func folderSizes(ctx context.Context, dir string, top int) (string, error) {
	if top <= 0 || top > 50 {
		top = 15
	}
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return dir + " is empty", nil
	}
	args := []string{"-xsk"}
	for _, e := range entries {
		args = append(args, filepath.Join(dir, e.Name()))
	}
	cmd := exec.CommandContext(ctx, "du", args...)
	out, _ := cmd.Output() // du returns non-zero on permission errors; keep partial output
	type row struct {
		kb   int64
		path string
	}
	var rows []row
	var total int64
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.SplitN(l, "\t", 2)
		if len(f) != 2 {
			continue
		}
		kb, err := strconv.ParseInt(f[0], 10, 64)
		if err != nil {
			continue
		}
		total += kb
		rows = append(rows, row{kb, f[1]})
	}
	if len(rows) == 0 {
		if ctx.Err() != nil {
			return "", fmt.Errorf("du timed out scanning %s; try a more specific sub-folder", dir)
		}
		return "", fmt.Errorf("could not measure %s", dir)
	}
	for i := 0; i < len(rows); i++ {
		for j := i + 1; j < len(rows); j++ {
			if rows[j].kb > rows[i].kb {
				rows[i], rows[j] = rows[j], rows[i]
			}
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Contents of %s, total %s, largest first:\n", dir, humanKB(total))
	for i, r := range rows {
		if i >= top {
			fmt.Fprintf(&b, "... and %d smaller entries\n", len(rows)-top)
			break
		}
		kind := ""
		if st, err := os.Lstat(r.path); err == nil && st.IsDir() {
			kind = "/"
		}
		pct := 0.0
		if total > 0 {
			pct = float64(r.kb) / float64(total) * 100
		}
		fmt.Fprintf(&b, "%8s  %4.1f%%  %s%s\n", humanKB(r.kb), pct, filepath.Base(r.path), kind)
	}
	if ctx.Err() != nil {
		b.WriteString("(scan was cut short by the timeout; sizes may be incomplete)\n")
	}
	return b.String(), nil
}
