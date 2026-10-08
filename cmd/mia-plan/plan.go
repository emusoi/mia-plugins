package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/emusoi/mia-plugins/plan"
	"github.com/emusoi/mia-plugins/records"
)

func (p plugin) planVerb(args []string) int {
	asJSON := take(&args, "--json")
	sub, wt, rest, err := p.subject(args, "show", "edit", "check", "path", "stack")
	if err != nil {
		return failed(err)
	}
	switch sub {
	case "path":
		if store := p.settings.Stores["plans"]; store != "" && store != "local" {
			return failed(fmt.Errorf("plans are kept in %s, not in a file — `mia plan %s edit` opens this one", store, wt.Name))
		}
		path := p.local().Path(records.Plans, p.key(wt.Branch))
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			if err := p.local().Put(records.Plans, p.key(wt.Branch), []byte(plan.Starter(wt.Branch))); err != nil {
				return failed(err)
			}
		}
		fmt.Println(path)
		return 0
	case "edit":
		return code(p.edit(records.Plans, p.key(wt.Branch), plan.Starter(wt.Branch)))
	case "check":
		return p.check(wt, strings.Join(rest, " "), asJSON)
	case "stack":
		return p.splitIntoLayers(wt)
	}
	pl, world, err := p.planFor(wt, p.base())
	if err != nil {
		return failed(err)
	}
	report := plan.Summarise(plan.Resolve(pl, world))
	if asJSON {
		return emit(struct {
			Path     string      `json:"path"`
			Worktree string      `json:"worktree"`
			Branch   string      `json:"branch"`
			Title    string      `json:"title,omitempty"`
			Report   plan.Report `json:"report"`
		}{p.local().Path(records.Plans, p.key(wt.Branch)), wt.Path, wt.Branch, pl.Title, report})
	}
	if report.Total == 0 {
		fmt.Printf("%s has no plan yet — `mia plan %s edit` writes one\n", wt.Name, wt.Name)
		return 0
	}
	if pl.Title != "" {
		fmt.Printf("%s   · %s\n", pl.Title, pl.Delivers())
	}
	section := plan.Section("")
	for _, one := range report.Steps {
		if one.Step.Section != section {
			section = one.Step.Section
			fmt.Printf("\n%s\n", strings.ToLower(string(section)))
		}
		check := ""
		if one.Step.Check != "" {
			check = " @" + one.Step.Check
		}
		fmt.Printf("  %s %-9s %s%s\n", mark(one.State), one.State, one.Step.Text, check)
		if one.Because != "" && one.State != plan.Proven {
			fmt.Printf("      %s\n", one.Because)
		}
	}
	fmt.Printf("\n%s\n", report.Line())
	return 0
}

func mark(state plan.State) string {
	switch state {
	case plan.Proven:
		return "✔"
	case plan.Attested:
		return "·"
	case plan.Stale:
		return "~"
	case plan.Claimed:
		return "?"
	default:
		return "☐"
	}
}

type checkRun struct {
	Step   plan.Step `json:"step"`
	Exit   int       `json:"exit"`
	Output string    `json:"-"`
	Where  string    `json:"where,omitempty"`
}

func (p plugin) check(wt worktree, only string, asJSON bool) int {
	if len(p.settings.Checks) == 0 {
		return failed(errors.New("no checks are configured — `[plugin.plan.checks]` in `mia config` names the commands a step can cite"))
	}
	base := p.base()
	pl, world, err := p.planFor(wt, base)
	if err != nil {
		return failed(err)
	}
	dirtyOut, _ := git(wt.Path, "status", "--porcelain", "-uno")
	evidence := p.records(records.Evidence)
	var ran, unknown int
	broken := false
	for _, step := range pl.Steps {
		if step.Check == "" || only != "" && !strings.Contains(plan.Normalise(step.Text), plan.Normalise(only)) {
			continue
		}
		argv, known := p.settings.Checks[step.Check]
		if !known || len(argv) == 0 {
			fmt.Fprintf(os.Stderr, "⚠ %s names @%s, which no `[plugin.plan.checks]` entry defines\n", step.Text, step.Check)
			unknown++
			continue
		}
		run := runCheck(wt, step, argv)
		line, err := plan.Line(plan.Proof{
			Key: step.Key(), Check: step.Check, Exit: run.Exit, SHA: world.Head, At: time.Now(),
			Output: tail(run.Output, 2000), Text: step.Text, Dirty: dirtyOut != "",
		})
		if err != nil {
			return failed(err)
		}
		if err := evidence.Append(records.Evidence, p.key(wt.Branch), line); err != nil {
			return failed(err)
		}
		ran++
		result := "passed"
		if run.Exit != 0 {
			broken, result = true, fmt.Sprintf("FAILED (%d)", run.Exit)
		}
		fmt.Printf("@%-10s %-8s %s · %s\n", step.Check, result, step.Text, run.Where)
		if run.Exit != 0 {
			fmt.Println(indent(clip(run.Output, 800)))
		}
	}
	if ran == 0 {
		switch {
		case unknown > 0:
			return 1
		case len(pl.Steps) == 0:
			fmt.Printf("%s has no plan yet — `mia plan %s edit` (p in the dashboard) writes one; its steps name the checks to run\n", wt.Name, wt.Name)
			return 0
		case only != "":
			for _, step := range pl.Steps {
				if strings.Contains(plan.Normalise(step.Text), plan.Normalise(only)) {
					fmt.Printf("nothing to run — %q names no check\n", step.Text)
					return 0
				}
			}
			return failed(fmt.Errorf("no step in %s's plan matches %q", wt.Name, only))
		}
		fmt.Println("nothing to run — no step in this plan names a check")
		return 0
	}
	pl, world, err = p.planFor(wt, base)
	if err != nil {
		return failed(err)
	}
	summary := plan.Summarise(plan.Resolve(pl, world))
	if asJSON {
		emit(summary)
	} else {
		fmt.Printf("\n%s\n", summary.Line())
	}
	if broken || unknown > 0 {
		return 1
	}
	return 0
}

func runCheck(wt worktree, step plan.Step, argv []string) checkRun {
	var out, said bytes.Buffer
	cmd := exec.Command("mia", append([]string{"env", "run", wt.Name}, argv...)...)
	cmd.Stdout, cmd.Stderr = &out, &said
	err := cmd.Run()
	exit := 0
	var exitErr *exec.ExitError
	switch {
	case errors.As(err, &exitErr):
		exit = exitErr.ExitCode()
	case err != nil:
		exit = 127
	}
	where := "in the worktree"
	text := said.String()
	if i := strings.LastIndex(text, "mia: ran "); i >= 0 {
		where = strings.TrimSpace(text[i+len("mia: ran "):])
		text = text[:i]
	}
	return checkRun{Step: step, Exit: exit, Output: out.String() + text, Where: where}
}

func (p plugin) splitIntoLayers(wt worktree) int {
	pl, _, err := p.planOf(wt.Path, wt.Branch, p.base())
	if err != nil {
		return failed(err)
	}
	if len(pl.Steps) == 0 {
		return failed(fmt.Errorf("%s has no plan — write the shape first: one step per layer", wt.Branch))
	}
	if dirty, _ := git(wt.Path, "status", "--porcelain"); dirty != "" {
		return failed(errors.New("there are uncommitted changes — commit them before splitting into layers"))
	}
	for _, step := range pl.Steps {
		name := layerName(step.Text)
		if _, err := git(wt.Path, "rev-parse", "--verify", "--quiet", "refs/heads/"+name); err == nil {
			fmt.Printf("%-24s already a branch, left alone\n", name)
			continue
		}
		if err := interactive("mia", "new", "--stack", "--in", wt.Name, name); err != nil {
			return failed(err)
		}
	}
	return 0
}

func (p plugin) edit(kind records.Kind, key, starter string) error {
	store := p.records(kind)
	body, err := store.Get(kind, key)
	if errors.Is(err, records.ErrAbsent) {
		body, err = []byte(starter), store.Put(kind, key, []byte(starter))
	}
	if err != nil {
		return err
	}
	if store.Name() == "local" {
		path := p.local().Path(kind, key)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return p.openEditor(path)
	}
	dir, err := os.MkdirTemp("", "mia-edit-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, filepath.Base(key)+".md")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return err
	}
	if err := p.openEditor(path); err != nil {
		return err
	}
	edited, err := os.ReadFile(path)
	if err != nil || bytes.Equal(edited, body) {
		return err
	}
	return store.Put(kind, key, edited)
}

func (p plugin) openEditor(path string) error {
	return interactive("sh", "-c", p.editor()+` "$1"`, "sh", path)
}

func take(args *[]string, flag string) bool {
	for i, arg := range *args {
		if arg == flag {
			*args = append((*args)[:i:i], (*args)[i+1:]...)
			return true
		}
	}
	return false
}

func emit(value any) int {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return failed(err)
	}
	fmt.Println(string(data))
	return 0
}

func failed(err error) int {
	fmt.Fprintf(os.Stderr, "mia: %v\n", err)
	return 1
}

func code(err error) int {
	if err != nil {
		return failed(err)
	}
	return 0
}

func tail(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	start := len(text) - limit
	for start < len(text) && !utf8.RuneStart(text[start]) {
		start++
	}
	return text[start:]
}

func indent(text string) string {
	var out []string
	for _, line := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		out = append(out, "    "+line)
	}
	return strings.Join(out, "\n")
}

func clip(text string, limit int) string {
	if cut := tail(text, limit); cut != text {
		return "…" + cut
	}
	return text
}
