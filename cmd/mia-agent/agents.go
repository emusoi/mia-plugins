package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type window struct {
	Index   int    `json:"index"`
	Name    string `json:"name"`
	Command string `json:"command"`
	PID     int    `json:"pid,omitempty"`
	Quiet   string `json:"quiet"`
}

type agent struct {
	Worktree string `json:"worktree"`
	Window   string `json:"window"`
	Index    int    `json:"index"`
	State    string `json:"state"`
	Quiet    string `json:"quiet"`
}

const (
	working  = "working"
	waiting  = "waiting"
	finished = "finished"
	idle     = "idle"
)

type memory struct {
	Last     string `json:"last"`
	Finished bool   `json:"finished,omitempty"`
	Hooked   bool   `json:"hooked,omitempty"`
}

func windowsOf(wt string) ([]window, error) {
	out, err := miaOutput("window", "ls", wt, "--json")
	if err != nil {
		return nil, err
	}
	var windows []window
	if err := json.Unmarshal(out, &windows); err != nil {
		return nil, fmt.Errorf("mia window ls: %w", err)
	}
	return windows, nil
}

func (p plugin) isAgentWindow(w window, table procs) bool {
	base, _, _ := strings.Cut(w.Name, "-")
	return p.isAgent(base) || table.agentAt(w.PID, p.bins()) != ""
}

func (p plugin) agentsIn(wt string) ([]agent, error) {
	windows, err := windowsOf(wt)
	if err != nil {
		return nil, err
	}
	return p.agentsFrom(wt, windows, loadProcs()), nil
}

func (p plugin) agentsFrom(wt string, windows []window, table procs) []agent {
	remembered := p.recall()
	changed := false
	var found []agent
	var mine []window
	for _, w := range windows {
		if !p.isAgentWindow(w, table) {
			continue
		}
		mine = append(mine, w)
		screen, _ := miaOutput("window", "read", wt, fmt.Sprint(w.Index))
		key := wt + "\x00" + w.Name
		before := remembered[key]
		seen := classify(string(screen))
		now := before
		if !before.Hooked {
			now = settle(before, seen)
		}
		if now != before {
			remembered[key] = now
			changed = true
		}
		state := shown(now)
		if seen == waiting {
			state = waiting
		}
		found = append(found, agent{Worktree: wt, Window: w.Name, Index: w.Index, State: state, Quiet: short(w.Quiet)})
	}
	live := map[string]bool{}
	for _, w := range mine {
		live[wt+"\x00"+w.Name] = true
	}
	for key := range remembered {
		if strings.HasPrefix(key, wt+"\x00") && !live[key] {
			delete(remembered, key)
			changed = true
		}
	}
	if changed {
		p.remember(remembered)
	}
	return found
}

func (p plugin) forget(wt, window string) {
	remembered := p.recall()
	if _, ok := remembered[wt+"\x00"+window]; ok {
		delete(remembered, wt+"\x00"+window)
		p.remember(remembered)
	}
}

func (p plugin) everyAgent() ([]agent, error) {
	out, err := miaOutput("api", "worktrees")
	if err != nil {
		return nil, err
	}
	var worktrees []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(out, &worktrees); err != nil {
		return nil, fmt.Errorf("mia api worktrees: %w", err)
	}
	table := loadProcs()
	var all []agent
	for _, wt := range worktrees {
		windows, err := windowsOf(wt.Name)
		if err != nil {
			continue
		}
		all = append(all, p.agentsFrom(wt.Name, windows, table)...)
	}
	return all, nil
}

var waitingMarks = []string{
	"Do you want to", "Would you like to", "❯ 1. Yes", "› 1. Yes", "(y/n)", "[y/N]", "[Y/n]",
	"Allow command", "Approve", "Press enter to continue",
}

var workingMarks = []string{"esc to interrupt", "Esc to interrupt", "ctrl+c to interrupt"}

func classify(screen string) string {
	var lines []string
	for _, line := range strings.Split(screen, "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) > 12 {
		lines = lines[len(lines)-12:]
	}
	bottom := strings.Join(lines, "\n")
	for _, mark := range waitingMarks {
		if strings.Contains(bottom, mark) {
			return waiting
		}
	}
	for _, mark := range workingMarks {
		if strings.Contains(bottom, mark) {
			return working
		}
	}
	return idle
}

func settle(before memory, now string) memory {
	switch {
	case now == idle && (before.Last == working || before.Finished):
		return memory{Last: idle, Finished: true}
	default:
		return memory{Last: now}
	}
}

func shown(m memory) string {
	if m.Last == idle && m.Finished {
		return finished
	}
	return m.Last
}

func (p plugin) see(wt, window string) {
	remembered := p.recall()
	key := wt + "\x00" + window
	if m, ok := remembered[key]; ok && m.Finished {
		m.Finished = false
		remembered[key] = m
		p.remember(remembered)
	}
}

func (p plugin) recall() map[string]memory {
	remembered := map[string]memory{}
	raw, err := os.ReadFile(p.statePath())
	if err == nil {
		_ = json.Unmarshal(raw, &remembered)
	}
	return remembered
}

func (p plugin) remember(remembered map[string]memory) {
	raw, err := json.Marshal(remembered)
	if err != nil {
		return
	}
	tmp := p.statePath() + ".tmp"
	if os.WriteFile(tmp, raw, 0o644) == nil {
		_ = os.Rename(tmp, p.statePath())
	}
}

func short(quiet string) string {
	d, err := time.ParseDuration(quiet)
	if err != nil {
		return quiet
	}
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}
