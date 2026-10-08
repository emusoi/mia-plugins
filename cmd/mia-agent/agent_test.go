package main

import "testing"

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
