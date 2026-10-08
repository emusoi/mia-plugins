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

func TestTheMostUrgentAgentNamesTheRow(t *testing.T) {
	info, ok := summarise([]agent{
		{Window: "claude", State: working, Quiet: "now"},
		{Window: "codex", State: waiting, Quiet: "4m"},
	})
	if !ok || info.Section != "waiting" || info.Status != "● waiting 4m" || len(info.Facts) != 2 {
		t.Fatalf("summarise = %+v", info)
	}
	if info, _ := summarise([]agent{{Window: "claude", State: idle}}); info.Section != "" || info.Status != "" {
		t.Errorf("an idle agent claims the row: %+v", info)
	}
}

func TestThenRunsTheNextNotifierWithThePayload(t *testing.T) {
	rest, then, err := splitThen([]string{"codex", "--then", `["/bin/sh", "-c", "printf %s \"$0\" > \"$OUT\""]`, `{"type":"agent-turn-complete"}`})
	if err != nil {
		t.Fatal(err)
	}
	if len(rest) != 2 || rest[0] != "codex" || len(then) != 3 {
		t.Fatalf("rest %q then %q", rest, then)
	}
	out := filepath.Join(t.TempDir(), "got")
	t.Setenv("OUT", out)
	if err := runThen(then, rest); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(out); string(got) != `{"type":"agent-turn-complete"}` {
		t.Errorf("next notifier got %q", got)
	}
	if _, _, err := splitThen([]string{"codex", "--then", "not json"}); err == nil {
		t.Error("a --then that is not a JSON list was accepted")
	}
	if rest, then, _ := splitThen([]string{"claude"}); len(then) != 0 || len(rest) != 1 {
		t.Errorf("no --then: rest %q then %q", rest, then)
	}
}

func TestAnAgentIsFoundByItsProcessUnderTheShell(t *testing.T) {
	table := parseProcs(`
  100     1 /bin/zsh -l
  101   100 claude
  102   101 /usr/bin/python3 mcp-server.py
  200     1 /bin/zsh -l
  201   200 node /opt/homebrew/bin/codex --full-auto
  300     1 /bin/zsh -l
  301   300 vim claude.md
`)
	bins := map[string]string{"claude": "claude", "codex": "codex"}
	for pid, want := range map[int]string{100: "claude", 101: "claude", 200: "codex", 300: "", 0: ""} {
		if got := table.agentAt(pid, bins); got != want {
			t.Errorf("agentAt(%d) = %q, want %q", pid, got, want)
		}
	}
}
