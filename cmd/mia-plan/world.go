package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/emusoi/mia-plugins/plan"
	"github.com/emusoi/mia-plugins/records"
)

type worktree struct {
	Name   string  `json:"name"`
	Path   string  `json:"path"`
	Branch string  `json:"branch,omitempty"`
	Main   bool    `json:"main,omitempty"`
	Layers []layer `json:"layers,omitempty"`
}

type layer struct {
	Branch   string `json:"branch"`
	Parent   string `json:"parent"`
	Landed   bool   `json:"landed"`
	Worktree string `json:"worktree,omitempty"`
}

func (p plugin) worktree(name string) (worktree, error) {
	path, err := p.pathOf(name)
	if err != nil {
		return worktree{}, err
	}
	out, err := mia("api", "worktrees")
	if err != nil {
		return worktree{}, err
	}
	var all []worktree
	if err := json.Unmarshal(out, &all); err != nil {
		return worktree{}, fmt.Errorf("mia api worktrees: %w", err)
	}
	for _, one := range all {
		if same(one.Path, path) {
			return one, nil
		}
	}
	return worktree{}, fmt.Errorf("no worktree %q", name)
}

func (p plugin) pathOf(name string) (string, error) {
	args := []string{"path"}
	if name != "" {
		args = append(args, name)
	}
	cmd := exec.Command("mia", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("no worktree %q", name)
	}
	return strings.TrimSpace(string(out)), nil
}

func same(a, b string) bool {
	if a == b {
		return true
	}
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && ra == rb
}

func (p plugin) isWorktree(word string) bool {
	_, err := p.pathOf(word)
	return err == nil
}

func (p plugin) subject(args []string, verbs ...string) (string, worktree, []string, error) {
	target := p.here
	if len(args) > 0 && !contains(verbs, args[0]) && p.isWorktree(args[0]) {
		target, args = args[0], args[1:]
	}
	sub := verbs[0]
	if len(args) > 0 {
		if !contains(verbs, args[0]) {
			return "", worktree{}, nil, fmt.Errorf("no %q here — one of %s", args[0], strings.Join(verbs, ", "))
		}
		sub, args = args[0], args[1:]
		if target == p.here && len(args) > 0 && p.isWorktree(args[0]) {
			target, args = args[0], args[1:]
		}
	}
	wt, err := p.worktree(target)
	if err != nil {
		return "", wt, nil, err
	}
	if wt.Branch == "" {
		return "", wt, nil, fmt.Errorf("%s is detached, and a plan belongs to a branch", wt.Name)
	}
	return sub, wt, args, nil
}

func contains(list []string, word string) bool {
	for _, one := range list {
		if one == word {
			return true
		}
	}
	return false
}

func slug(branch string) string {
	var b strings.Builder
	for _, r := range branch {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return b.String() + "-" + plan.Key(branch)[:8]
}

func (p plugin) key(branch string) string { return slug(filepath.Base(p.repo)) + "/" + slug(branch) }

func (p plugin) local() records.Local { return records.Local{Dir: p.miaDir} }

func (p plugin) records(kind records.Kind) records.Store {
	name := p.settings.Stores[string(kind)]
	if name == "" || name == "local" {
		return p.local()
	}
	helper, err := records.Find(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mia: %v — using .git/mia/ for %s\n", err, kind)
		return p.local()
	}
	return &records.Falling{To: helper, Local: p.local(), Warn: os.Stderr}
}

func (p plugin) base() string {
	out, err := mia("api", "base")
	if err != nil {
		return "main"
	}
	var base struct {
		Branch string `json:"branch"`
	}
	if json.Unmarshal(out, &base) != nil || base.Branch == "" {
		return "main"
	}
	return base.Branch
}

func (p plugin) planOf(dir, branch, base string) (plan.Plan, plan.World, error) {
	markdown, err := p.records(records.Plans).Get(records.Plans, p.key(branch))
	if err != nil && !errors.Is(err, records.ErrAbsent) {
		return plan.Plan{}, plan.World{}, err
	}
	lines, err := p.records(records.Evidence).Get(records.Evidence, p.key(branch))
	if err != nil && !errors.Is(err, records.ErrAbsent) {
		return plan.Plan{}, plan.World{}, err
	}
	head, _ := git(dir, "rev-parse", branch)
	var subjects []string
	if out, err := git(dir, "log", "--format=%s", base+".."+branch); err == nil && out != "" {
		subjects = strings.Split(out, "\n")
	}
	return plan.Parse(string(markdown)), plan.World{Head: head, Subjects: subjects, Proofs: plan.LatestIn(lines)}, nil
}

func (p plugin) planFor(wt worktree, base string) (plan.Plan, plan.World, error) {
	pl, world, err := p.planOf(wt.Path, wt.Branch, base)
	if err != nil || len(wt.Layers) < 2 || wt.Layers[0].Branch != wt.Branch {
		return pl, world, err
	}
	world.Layers = map[string]plan.LayerFact{}
	for _, one := range wt.Layers[1:] {
		lp, lw, err := p.planOf(wt.Path, one.Branch, base)
		if err != nil {
			continue
		}
		report := plan.Summarise(plan.Resolve(lp, lw))
		for _, step := range pl.Steps {
			if strings.HasSuffix(one.Branch, layerName(step.Text)) {
				world.Layers[step.Key()] = plan.LayerFact{Branch: one.Branch, Landed: one.Landed, Proven: report.Proven, Total: report.Total}
			}
		}
	}
	return pl, world, nil
}

func layerName(text string) string {
	head := text
	for _, cut := range []string{" — ", " - ", ": ", ", "} {
		if index := strings.Index(head, cut); index > 0 {
			head = head[:index]
			break
		}
	}
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(head)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ', r == '_', r == '-':
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
