package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"
)

const manifest = `{
  "protocol": 1,
  "help": "coding agents in your worktrees, and which of them is waiting on you",
  "verbs": [
    {"name": "agent", "usage": "mia agent [ls|run [worktree] [--task <text>] [agent]|attach [worktree] [window]|send [worktree] [window] -- <text>|stop [worktree] [window]|hooks <install|uninstall>]", "help": "start an agent in a worktree's session, and see which agents are working, waiting or finished"}
  ]
}`

type settings struct {
	Default string            `json:"default"`
	Agents  []string          `json:"agents"`
	Bin     map[string]string `json:"bin"`
}

type plugin struct {
	here     string
	data     string
	settings settings
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "mia-agent is a mia plugin: `mia plugin enable agent`, then `mia agent run`")
		os.Exit(64)
	}
	if os.Args[1] == "manifest" {
		fmt.Println(manifest)
		return
	}
	p, err := load()
	if err != nil {
		fail(err)
	}
	switch os.Args[1] {
	case "agent":
		err = p.verb(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "mia-agent: no %q\n", os.Args[1])
		os.Exit(64)
	}
	if err != nil {
		fail(err)
	}
}

func load() (plugin, error) {
	p := plugin{here: os.Getenv("MIA_WORKTREE"), data: os.Getenv("MIA_PLUGIN_DATA")}
	if p.data == "" {
		return p, errors.New("MIA_PLUGIN_DATA is not set — run this through mia")
	}
	if raw := os.Getenv("MIA_PLUGIN_CONFIG"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &p.settings); err != nil {
			return p, fmt.Errorf("[plugin.agent]: %w", err)
		}
	}
	if p.settings.Default == "" {
		p.settings.Default = "claude"
	}
	if len(p.settings.Agents) == 0 {
		p.settings.Agents = []string{"claude", "codex"}
	}
	return p, nil
}

func (p plugin) verb(args []string) error {
	sub := "ls"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		switch args[0] {
		case "ls", "run", "attach", "send", "stop", "hook", "hooks":
			sub, args = args[0], args[1:]
		}
	}
	switch sub {
	case "ls":
		return p.list(args)
	case "hook":
		return p.hook(args, os.Stdin)
	case "hooks":
		return p.hooks(args)
	case "run":
		return p.run(args)
	case "attach":
		wt, window, _, err := p.aim(args)
		if err != nil {
			return err
		}
		p.see(wt, window)
		return interactive("mia", "window", "open", wt, window)
	case "send":
		wt, window, text, err := p.aim(args)
		if err != nil {
			return err
		}
		if text == "" {
			return errors.New("usage: mia agent send [worktree] [window] -- <text>")
		}
		return interactive("mia", "window", "send", wt, window, "--", text)
	case "stop":
		wt, window, _, err := p.aim(args)
		if err != nil {
			return err
		}
		return interactive("mia", "window", "close", wt, window)
	}
	return nil
}

func (p plugin) list(args []string) error {
	asJSON := len(args) == 1 && args[0] == "--json"
	if len(args) > 0 && !asJSON {
		return errors.New("usage: mia agent ls [--json]")
	}
	all, err := p.everyAgent()
	if err != nil {
		return err
	}
	if asJSON {
		return json.NewEncoder(os.Stdout).Encode(all)
	}
	if len(all) == 0 {
		fmt.Println("no agents — `mia agent run` starts one")
		return nil
	}
	out := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, a := range all {
		fmt.Fprintf(out, "%s\t%s\t%s\t%s\n", a.Worktree, a.Window, a.State, a.Quiet)
	}
	return out.Flush()
}

func (p plugin) run(args []string) error {
	var task, wt, agent string
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--task" && i+1 < len(args):
			task = args[i+1]
			i++
		case strings.HasPrefix(args[i], "-"):
			return errors.New("usage: mia agent run [worktree] [--task <text>] [agent]")
		case p.isAgent(args[i]) && agent == "":
			agent = args[i]
		case wt == "":
			wt = args[i]
		default:
			return errors.New("usage: mia agent run [worktree] [--task <text>] [agent]")
		}
	}
	if agent == "" {
		agent = p.settings.Default
	}
	if wt == "" {
		wt = p.here
	}
	if wt == "" {
		return errors.New("which worktree? `mia agent run <worktree>`")
	}
	windows, err := windowsOf(wt)
	if err != nil {
		return err
	}
	window := freeName(agent, windows)
	argv := []string{"mia", "window", "new", wt, window, "--", p.bin(agent)}
	if task != "" {
		argv = append(argv, task)
	}
	return interactive(argv[0], argv[1:]...)
}

func (p plugin) isAgent(name string) bool {
	for _, a := range p.settings.Agents {
		if a == name {
			return true
		}
	}
	return false
}

func (p plugin) bin(agent string) string {
	if bin := p.settings.Bin[agent]; bin != "" {
		return bin
	}
	return agent
}

func freeName(agent string, windows []window) string {
	taken := map[string]bool{}
	for _, w := range windows {
		taken[w.Name] = true
	}
	name := agent
	for n := 2; taken[name]; n++ {
		name = fmt.Sprintf("%s-%d", agent, n)
	}
	return name
}

func (p plugin) aim(args []string) (wt, window, text string, err error) {
	if i := indexOf(args, "--"); i >= 0 {
		text, args = strings.Join(args[i+1:], " "), args[:i]
	}
	switch len(args) {
	case 0:
		wt = p.here
	case 1:
		if p.isAgent(strings.SplitN(args[0], "-", 2)[0]) {
			wt, window = p.here, args[0]
		} else {
			wt = args[0]
		}
	case 2:
		wt, window = args[0], args[1]
	default:
		return "", "", "", errors.New("too many arguments")
	}
	if wt == "" {
		return "", "", "", errors.New("which worktree?")
	}
	if window != "" {
		return wt, window, text, nil
	}
	agents, err := p.agentsIn(wt)
	if err != nil {
		return "", "", "", err
	}
	switch len(agents) {
	case 0:
		return "", "", "", fmt.Errorf("%s has no agent — `mia agent run %s`", wt, wt)
	case 1:
		return wt, agents[0].Window, text, nil
	}
	names := make([]string, len(agents))
	for i, a := range agents {
		names[i] = a.Window
	}
	return "", "", "", fmt.Errorf("%s has %d agents (%s) — name one", wt, len(agents), strings.Join(names, ", "))
}

func indexOf(args []string, want string) int {
	for i, a := range args {
		if a == want {
			return i
		}
	}
	return -1
}

func interactive(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

func miaOutput(args ...string) ([]byte, error) {
	cmd := exec.Command("mia", args...)
	out, err := cmd.Output()
	var exit *exec.ExitError
	if errors.As(err, &exit) && len(exit.Stderr) > 0 {
		return nil, errors.New(strings.TrimSpace(strings.TrimPrefix(string(exit.Stderr), "mia: ")))
	}
	return out, err
}

func (p plugin) statePath() string { return filepath.Join(p.data, "agents.json") }

func fail(err error) {
	fmt.Fprintf(os.Stderr, "mia: %v\n", err)
	os.Exit(1)
}
