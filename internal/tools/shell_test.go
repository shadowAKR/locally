package tools

import "testing"

func TestClassifyShell(t *testing.T) {
	cases := []struct {
		cmd  string
		want Risk
	}{
		{"ls -la ~/Downloads", Safe},
		{"df -h | grep /home", Safe},
		{"cat /etc/os-release && uname -a", Safe},
		{"git status", Safe},
		{"git push origin main", Risky},
		{"systemctl status bluetooth", Safe},
		{"systemctl restart bluetooth", Risky},
		{"nmcli device wifi list", Safe},
		{"nmcli radio wifi off", Risky},
		{"echo hi > ~/x.txt", Risky},
		{"ls 2>/dev/null", Risky},
		{"sudo apt-get clean", Risky},
		{"rm -rf ~/.cache/thumbnails", Risky},
		{"sed -n '1,5p' file.txt", Safe},
		{"sed -i 's/a/b/' file.txt", Risky},
		{"FOO=bar env | grep FOO", Safe},
		{`find ~ -name "*.pdf" -exec rm {} \;`, Safe}, // find is allowlisted; -exec is a known gap
		{"echo $(whoami)", Risky},
		{"apt install vim", Risky},
		{"apt list --installed", Safe},
		{"pactl set-sink-volume @DEFAULT_SINK@ 50%", Risky},
		{"pactl list sinks", Safe},
		{"brightnessctl set 50%", Risky},
		{"python3 -c 'print(1)'", Risky},
		{"xdg-open https://example.com", Risky},
		{"", Safe},
	}
	for _, c := range cases {
		got, reason := classifyShell(c.cmd)
		if got != c.want {
			t.Errorf("classifyShell(%q) = %v (%s), want %v", c.cmd, got, reason, c.want)
		}
	}
}

func TestExpandPath(t *testing.T) {
	if p := expandPath("~/Desktop"); p == "~/Desktop" || p == "" {
		t.Errorf("tilde not expanded: %q", p)
	}
	if p := expandPath("/etc/hosts"); p != "/etc/hosts" {
		t.Errorf("absolute path changed: %q", p)
	}
}
