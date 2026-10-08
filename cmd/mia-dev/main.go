package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"time"
)

const manifest = `{
  "protocol": 1,
  "help": "lend the main checkout's running dev server a worktree's files, uncommitted work included",
  "verbs": [
    {"name": "dev", "usage": "mia dev [<worktree> [--follow]|off]", "help": "lend the main checkout's dev server a worktree's files; off gives main its own back"}
  ],
  "rows": {"every": "5s"},
  "keys": [
    {"id": "lend", "key": "v", "label": "on main's dev server", "verb": "dev", "args": ["{row}"], "report": true,
     "help": "copy the worktree's files over the main checkout, so its running dev server shows them; v on main gives it back"}
  ]
}`

type lent struct {
	Worktree string `json:"worktree"`
	Path     string `json:"path"`
	Stashed  bool   `json:"stashed"`
}

type plugin struct {
	main  string
	state string
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "mia-dev is a mia plugin: `mia plugin enable dev`, then `mia dev <worktree>`")
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
	case "dev":
		err = p.verb(os.Args[2:])
	case "rows":
		err = p.rows(os.Stdin, os.Stdout)
	default:
		fmt.Fprintf(os.Stderr, "mia-dev: no %q\n", os.Args[1])
		os.Exit(64)
	}
	if err != nil {
		fail(err)
	}
}

func load() (plugin, error) {
	data := os.Getenv("MIA_PLUGIN_DATA")
	if data == "" {
		return plugin{}, errors.New("MIA_PLUGIN_DATA is not set — run this through mia")
	}
	main, err := miaPath("")
	if err != nil {
		return plugin{}, err
	}
	return plugin{main: main, state: filepath.Join(data, "lent.json")}, nil
}

func (p plugin) verb(args []string) error {
	follow := false
	var target string
	for _, arg := range args {
		switch {
		case arg == "--follow":
			follow = true
		case target == "" && !strings.HasPrefix(arg, "-"):
			target = arg
		default:
			return fmt.Errorf("usage: mia dev [<worktree> [--follow]|off]")
		}
	}
	if target == "" {
		return p.status()
	}
	if target == "off" {
		return p.off()
	}
	path, err := miaPath(target)
	if err != nil {
		return err
	}
	if same(path, p.main) {
		return p.off()
	}
	if err := p.lend(target, path); err != nil {
		return err
	}
	fmt.Printf("main's dev server now has %s's files — `mia dev off` gives main its own back\n", target)
	if follow {
		return p.follow(path)
	}
	return nil
}

func (p plugin) status() error {
	current, ok, err := p.read()
	if err != nil {
		return err
	}
	if !ok {
		fmt.Println("main's dev server has main's own files")
		return nil
	}
	fmt.Printf("main's dev server has %s's files — `mia dev off` gives main its own back\n", current.Worktree)
	return nil
}

func (p plugin) lend(name, path string) error {
	current, ok, err := p.read()
	if err != nil {
		return err
	}
	if ok && current.Worktree != name {
		if err := p.off(); err != nil {
			return err
		}
		ok = false
	}
	if !ok {
		current = lent{Worktree: name, Path: path}
		dirty, err := git(p.main, "status", "--porcelain")
		if err != nil {
			return err
		}
		if dirty != "" {
			if _, err := git(p.main, "stash", "push", "--include-untracked", "-m", stashMessage); err != nil {
				return fmt.Errorf("could not put main's own work aside: %w", err)
			}
			current.Stashed = true
		}
		if err := p.write(current); err != nil {
			return err
		}
	}
	_, err = mirror(path, p.main)
	return err
}

func (p plugin) follow(path string) error {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	fmt.Println("following — ctrl-c stops; main keeps the files until `mia dev off`")
	for {
		select {
		case <-stop:
			return nil
		case <-tick.C:
			changed, err := mirror(path, p.main)
			if err != nil {
				return err
			}
			if changed > 0 {
				fmt.Printf("%s  %d changed\n", time.Now().Format("15:04:05"), changed)
			}
		}
	}
}

const stashMessage = "mia dev: main's own work"

func (p plugin) off() error {
	current, ok, err := p.read()
	if err != nil {
		return err
	}
	if !ok {
		fmt.Println("main's dev server already has main's own files")
		return nil
	}
	if _, err := git(p.main, "reset", "-q", "--hard"); err != nil {
		return err
	}
	if _, err := git(p.main, "clean", "-fdq"); err != nil {
		return err
	}
	if current.Stashed {
		ref, err := stashRef(p.main)
		if err != nil {
			return err
		}
		if ref == "" {
			return fmt.Errorf("main's own work was put aside but its stash is gone — look in `git stash list`")
		}
		if _, err := git(p.main, "stash", "pop", "-q", ref); err != nil {
			return fmt.Errorf("main's own work is still in %s: %w", ref, err)
		}
	}
	if err := os.Remove(p.state); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	fmt.Printf("main has its own files back (was %s's)\n", current.Worktree)
	return nil
}

func stashRef(dir string) (string, error) {
	list, err := git(dir, "stash", "list", "--format=%gd %s")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(list, "\n") {
		ref, subject, _ := strings.Cut(line, " ")
		if strings.HasSuffix(subject, stashMessage) {
			return ref, nil
		}
	}
	return "", nil
}

func (p plugin) read() (lent, bool, error) {
	raw, err := os.ReadFile(p.state)
	if errors.Is(err, os.ErrNotExist) {
		return lent{}, false, nil
	}
	if err != nil {
		return lent{}, false, err
	}
	var current lent
	if err := json.Unmarshal(raw, &current); err != nil {
		return lent{}, false, fmt.Errorf("%s: %w", p.state, err)
	}
	return current, true, nil
}

func (p plugin) write(current lent) error {
	raw, err := json.Marshal(current)
	if err != nil {
		return err
	}
	return os.WriteFile(p.state, raw, 0o644)
}

type rowInfo struct {
	Facts  []string `json:"facts,omitempty"`
	Status string   `json:"status,omitempty"`
}

func (p plugin) rows(in io.Reader, out io.Writer) error {
	var input struct {
		Worktrees []struct {
			Name string `json:"name"`
			Path string `json:"path"`
			Main bool   `json:"main,omitempty"`
		} `json:"worktrees"`
	}
	if err := json.NewDecoder(in).Decode(&input); err != nil {
		return fmt.Errorf("rows: %w", err)
	}
	answer := map[string]rowInfo{}
	if current, ok, err := p.read(); err == nil && ok {
		for _, wt := range input.Worktrees {
			switch {
			case wt.Main:
				answer[wt.Name] = rowInfo{Status: "has " + current.Worktree + "'s files", Facts: []string{"dev     has " + current.Worktree + "'s files"}}
			case wt.Name == current.Worktree:
				answer[wt.Name] = rowInfo{Status: "on main's dev server", Facts: []string{"dev     on main's dev server"}}
			}
		}
	}
	return json.NewEncoder(out).Encode(struct {
		Rows map[string]rowInfo `json:"rows"`
	}{answer})
}

func miaPath(name string) (string, error) {
	args := []string{"path"}
	if name != "" {
		args = append(args, name)
	}
	out, err := exec.Command("mia", args...).Output()
	if err != nil {
		if name == "" {
			return "", errors.New("mia path: no main checkout")
		}
		return "", fmt.Errorf("no worktree %q", name)
	}
	return strings.TrimSpace(string(out)), nil
}

func same(a, b string) bool {
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && ra == rb
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "mia: %v\n", err)
	os.Exit(1)
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && len(exit.Stderr) > 0 {
			return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(exit.Stderr)))
		}
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}
