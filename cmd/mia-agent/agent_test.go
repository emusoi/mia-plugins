package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestClassifyReadsTheBottomOfTheScreen(t *testing.T) {
	for screen, want := range map[string]string{
		"✻ Thinking… (12s · esc to interrupt)\n":                        working,
		"Do you want to make this edit to app.go?\n❯ 1. Yes\n  2. No\n": waiting,
		"> \n  ? for shortcuts\n":                                       idle,
		"Do you want to proceed?\n" + lines(30) + "> ":                  idle,
	} {
		if got := classify(screen); got != want {
			t.Errorf("classify(%.30q) = %s, want %s", screen, got, want)
		}
	}
}

func lines(n int) string {
	s := ""
	for range n {
		s += "output\n"
	}
	return s
}

func TestFinishedLastsUntilSeen(t *testing.T) {
	m := settle(memory{}, working)
	if shown(m) != working {
		t.Fatalf("shown = %s", shown(m))
	}
	m = settle(m, idle)
	if shown(m) != finished {
		t.Fatalf("after working then idle, shown = %s, want finished", shown(m))
	}
	m = settle(m, idle)
	if shown(m) != finished {
		t.Fatalf("still idle, shown = %s, want finished until seen", shown(m))
	}
	m.Finished = false
	if shown(settle(m, idle)) != idle {
		t.Fatalf("seen, then idle: want idle")
	}
	if shown(settle(memory{}, idle)) != idle {
		t.Fatalf("never worked: want idle")
	}
}

func TestFreeName(t *testing.T) {
	if got := freeName("claude", []window{{Name: "shell"}, {Name: "claude"}, {Name: "claude-2"}}); got != "claude-3" {
		t.Errorf("freeName = %s", got)
	}
	if got := freeName("codex", nil); got != "codex" {
		t.Errorf("freeName = %s", got)
	}
}

func TestHookStates(t *testing.T) {
	for _, c := range []struct{ name, message, kind, want string }{
		{"UserPromptSubmit", "", "", working},
		{"PostToolUse", "", "", working},
		{"Notification", "Claude needs your permission to use Bash", "", waiting},
		{"Notification", "Claude is waiting for your input", "", ""},
		{"Stop", "", "", finished},
		{"", "", "agent-turn-complete", finished},
	} {
		if got, _ := hookState(c.name, c.message, c.kind); got != c.want {
			t.Errorf("hookState(%s, %q, %s) = %q, want %q", c.name, c.message, c.kind, got, c.want)
		}
	}
	if _, forget := hookState("SessionEnd", "", ""); !forget {
		t.Error("SessionEnd should forget the agent")
	}
	if shown(hooked(finished)) != finished {
		t.Error("a hooked finish shows finished")
	}
}

func TestClaudeHooksInstallTwiceAndUninstallKeepTheirs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	theirs := `{"model":"opus","hooks":{"Stop":[{"hooks":[{"type":"command","command":"say done"}]}]}}`
	if err := os.WriteFile(path, []byte(theirs), 0o644); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := editClaude(path, true); err != nil {
			t.Fatal(err)
		}
	}
	var settings struct {
		Model string `json:"model"`
		Hooks map[string][]struct {
			Hooks []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	read := func() {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		settings.Hooks = nil
		if err := json.Unmarshal(raw, &settings); err != nil {
			t.Fatal(err)
		}
	}
	read()
	if settings.Model != "opus" || len(settings.Hooks["Stop"]) != 2 || len(settings.Hooks["UserPromptSubmit"]) != 1 {
		t.Fatalf("after installing twice: %+v", settings)
	}
	if err := editClaude(path, false); err != nil {
		t.Fatal(err)
	}
	read()
	if len(settings.Hooks) != 1 || settings.Hooks["Stop"][0].Hooks[0].Command != "say done" {
		t.Fatalf("after uninstall: %+v", settings.Hooks)
	}
}
