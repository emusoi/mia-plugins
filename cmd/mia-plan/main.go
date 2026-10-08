package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const manifest = `{
  "protocol": 1,
  "help": "plans whose steps are proven by checks, and lessons, per branch",
  "verbs": [
    {"name": "plan", "usage": "mia plan [worktree] [show|edit|check [step]|path|stack]", "help": "what this branch is doing, and what is actually proven"},
    {"name": "lesson", "usage": "mia lesson [worktree] [list|open|edit [name]|path [name]]", "help": "notes for this branch: markdown files in a folder, in your editor"}
  ],
  "rows": {"every": "10s"},
  "sections": [
    {"id": "needs-proof", "label": "needs proof", "rank": 25},
    {"id": "proven", "label": "proven", "rank": 28}
  ],
  "keys": [
    {"id": "check", "key": "C", "label": "check the plan", "verb": "plan", "args": ["{row}", "check"], "report": true,
     "help": "run every check the plan cites, record the proof, and show which steps are proven"},
    {"id": "edit", "key": "p", "label": "plan", "verb": "plan", "args": ["{row}", "edit"], "popup": true, "lands": true,
     "help": "open the branch's plan in your editor; C gathers proof"},
    {"id": "lessons", "key": "L", "label": "lessons", "verb": "lesson", "args": ["{row}", "open"], "popup": true, "lands": true,
     "help": "the branch's notes folder in your editor"}
  ]
}`

type settings struct {
	Checks map[string][]string `json:"checks"`
	Stores map[string]string   `json:"stores"`
	Editor string              `json:"editor"`
}

type plugin struct {
	repo     string
	miaDir   string
	here     string
	settings settings
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "mia-plan is a mia plugin: `mia plugin enable plan`, then `mia plan`")
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
	args := os.Args[2:]
	switch os.Args[1] {
	case "plan":
		os.Exit(p.planVerb(args))
	case "lesson":
		os.Exit(p.lessonVerb(args))
	case "rows":
		if err := p.rows(os.Stdin, os.Stdout); err != nil {
			fail(err)
		}
	default:
		fmt.Fprintf(os.Stderr, "mia-plan: no %q\n", os.Args[1])
		os.Exit(64)
	}
}

func load() (plugin, error) {
	p := plugin{repo: os.Getenv("MIA_REPO"), here: os.Getenv("MIA_WORKTREE")}
	if p.repo == "" {
		return p, errors.New("MIA_REPO is not set — run this through mia")
	}
	if raw := os.Getenv("MIA_PLUGIN_CONFIG"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &p.settings); err != nil {
			return p, fmt.Errorf("[plugin.plan]: %w", err)
		}
	}
	common, err := git(p.repo, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return p, err
	}
	p.miaDir = filepath.Join(common, "mia")
	return p, nil
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "mia: %v\n", err)
	os.Exit(1)
}

func git(dir string, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	return strings.TrimSpace(string(out)), err
}

func mia(args ...string) ([]byte, error) {
	cmd := exec.Command("mia", args...)
	cmd.Stderr = os.Stderr
	return cmd.Output()
}

func (p plugin) editor() string {
	for _, candidate := range []string{p.settings.Editor, os.Getenv("VISUAL"), os.Getenv("EDITOR")} {
		if candidate != "" {
			return candidate
		}
	}
	return "vi"
}

func interactive(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}
