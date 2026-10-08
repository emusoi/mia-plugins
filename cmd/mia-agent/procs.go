package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
)

type procs struct {
	children map[int][]int
	names    map[int][]string
}

func loadProcs() procs {
	out, err := exec.Command("ps", "-A", "-o", "pid=,ppid=,args=").Output()
	if err != nil {
		return procs{}
	}
	return parseProcs(string(out))
}

func parseProcs(table string) procs {
	t := procs{children: map[int][]int{}, names: map[int][]string{}}
	for _, line := range strings.Split(table, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		pid, err1 := strconv.Atoi(fields[0])
		ppid, err2 := strconv.Atoi(fields[1])
		if err1 != nil || err2 != nil {
			continue
		}
		t.children[ppid] = append(t.children[ppid], pid)
		for _, arg := range fields[2:min(len(fields), 4)] {
			t.names[pid] = append(t.names[pid], filepath.Base(arg))
		}
	}
	return t
}

func (t procs) agentAt(pid int, bins map[string]string) string {
	if pid <= 0 {
		return ""
	}
	level := []int{pid}
	for depth := 0; depth < 3 && len(level) > 0; depth++ {
		var next []int
		for _, p := range level {
			for _, name := range t.names[p] {
				if agent, ok := bins[name]; ok {
					return agent
				}
			}
			next = append(next, t.children[p]...)
		}
		level = next
	}
	return ""
}

func (p plugin) bins() map[string]string {
	bins := map[string]string{}
	for _, a := range p.settings.Agents {
		bins[filepath.Base(p.bin(a))] = a
	}
	return bins
}

type pane struct {
	ID      string `json:"pane"`
	Session string `json:"session"`
	Index   int    `json:"index"`
	Window  string `json:"window"`
	PID     int    `json:"-"`
	Path    string `json:"path"`
	Agent   string `json:"agent"`
	State   string `json:"state"`
}

func (pn pane) target() string { return fmt.Sprintf("%s:%d", pn.Session, pn.Index) }

const sep = " :mia: "

func everyPane() []pane {
	out, err := exec.Command("tmux", "list-panes", "-a", "-F",
		"#{pane_id}"+sep+"#{session_name}"+sep+"#{window_index}"+sep+"#{window_name}"+sep+"#{pane_pid}"+sep+"#{pane_current_path}").Output()
	if err != nil {
		return nil
	}
	var panes []pane
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.Split(line, sep)
		if len(f) < 6 {
			continue
		}
		index, _ := strconv.Atoi(f[2])
		pid, _ := strconv.Atoi(f[4])
		panes = append(panes, pane{ID: f[0], Session: f[1], Index: index, Window: f[3], PID: pid, Path: f[5]})
	}
	return panes
}

func (p plugin) everywhere() []pane {
	table, bins := loadProcs(), p.bins()
	var found []pane
	for _, pn := range everyPane() {
		if pn.Agent = table.agentAt(pn.PID, bins); pn.Agent == "" {
			continue
		}
		screen, _ := exec.Command("tmux", "capture-pane", "-p", "-t", pn.ID).Output()
		pn.State = classify(string(screen))
		found = append(found, pn)
	}
	sort.SliceStable(found, func(i, j int) bool { return urgency[found[i].State] < urgency[found[j].State] })
	return found
}

func (p plugin) listEverywhere(asJSON bool) error {
	found := p.everywhere()
	if asJSON {
		return writeJSON(found)
	}
	if len(found) == 0 {
		fmt.Println("no agents in any tmux session")
		return nil
	}
	home, _ := os.UserHomeDir()
	out := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, pn := range found {
		path := pn.Path
		if home != "" && strings.HasPrefix(path, home) {
			path = "~" + strings.TrimPrefix(path, home)
		}
		fmt.Fprintf(out, "%s\t%s\t%s\t%s\n", pn.target(), pn.Agent, pn.State, path)
	}
	return out.Flush()
}

func attachTarget(target string) error {
	if err := exec.Command("tmux", "select-window", "-t", target).Run(); err != nil {
		return fmt.Errorf("no tmux window %s — `mia agent ls --all` lists them", target)
	}
	if os.Getenv("TMUX") != "" {
		return interactive("tmux", "switch-client", "-t", target)
	}
	return interactive("tmux", "attach-session", "-t", target)
}

func (p plugin) everywherePanel() map[string]any {
	home, _ := os.UserHomeDir()
	var rows []map[string]any
	for _, pn := range p.everywhere() {
		path := pn.Path
		if home != "" && strings.HasPrefix(path, home) {
			path = "~" + strings.TrimPrefix(path, home)
		}
		rows = append(rows, map[string]any{
			"id":      pn.target(),
			"glyph":   looks[pn.State].glyph,
			"cells":   []string{pn.target(), pn.Agent, pn.State, path},
			"actions": []string{"attach"},
			"facts":   []string{"session " + pn.Session, "window  " + pn.Window, "path    " + path},
		})
	}
	return map[string]any{
		"version":  2,
		"id":       "everywhere",
		"title":    "Agents / every tmux session",
		"columns":  []map[string]string{{"Name": "where"}, {"Name": "agent"}, {"Name": "state"}, {"Name": "path"}},
		"sections": []map[string]any{{"id": "agents", "label": "agents", "rows": rows}},
		"actions": map[string]any{
			"attach": map[string]any{"key": "⏎", "label": "land there", "verb": "agent", "args": []string{"attach", "{row}"}, "lands": true,
				"help": "switch to the agent's tmux window, in whatever session it runs"},
		},
		"hints": []string{"⏎ land there"},
		"empty": "No agents in any tmux session. `mia agent run` starts one.",
	}
}
