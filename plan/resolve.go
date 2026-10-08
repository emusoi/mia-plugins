package plan

import (
	"fmt"
	"strings"
)

type State string

const (
	Proven   State = "proven"
	Stale    State = "stale"
	Attested State = "attested"
	Claimed  State = "claimed"
	Open     State = "open"
)

type Resolved struct {
	Step    Step   `json:"step"`
	State   State  `json:"state"`
	Because string `json:"because"`
}

type World struct {
	Head     string
	Subjects []string
	Proofs   map[string]Proof
	Layers   map[string]LayerFact
}

type LayerFact struct {
	Branch string
	Landed bool
	Proven int
	Total  int
}

func Resolve(p Plan, world World) []Resolved {
	out := make([]Resolved, 0, len(p.Steps))
	for _, step := range p.Steps {
		out = append(out, resolveStep(step, world))
	}
	return out
}

func resolveStep(step Step, world World) Resolved {
	if layer, ok := world.Layers[step.Key()]; ok {
		switch {
		case layer.Landed:
			return Resolved{step, Proven, "layer " + layer.Branch + " has landed"}
		case layer.Total > 0 && layer.Proven == layer.Total:
			return Resolved{step, Attested, fmt.Sprintf("layer %s: every step proven, not landed yet", layer.Branch)}
		case layer.Total > 0:
			return Resolved{step, Open, fmt.Sprintf("layer %s: %d/%d proven", layer.Branch, layer.Proven, layer.Total)}
		default:
			return Resolved{step, Open, "layer " + layer.Branch + " has no plan yet"}
		}
	}
	proof, hasProof := world.Proofs[step.Key()]

	if hasProof && proof.Exit == 0 && proof.SHA == world.Head && world.Head != "" && proof.Dirty {
		return Resolved{step, Stale, fmt.Sprintf("@%s passed on uncommitted changes — commit them, then check again", proof.Check)}
	}
	if hasProof && proof.Exit == 0 && proof.SHA == world.Head && world.Head != "" {
		return Resolved{step, Proven, fmt.Sprintf("@%s passed at this commit", proof.Check)}
	}
	if hasProof && proof.Exit != 0 && proof.SHA == world.Head && world.Head != "" {
		return Resolved{step, Open, fmt.Sprintf("@%s failed at this commit", proof.Check)}
	}
	if step.Check == "" && step.Section != Review {
		if subject := matchingCommit(step, world.Subjects); subject != "" {
			return Resolved{step, Proven, "a commit says so: " + subject}
		}
	}
	if hasProof && proof.Exit == 0 {
		return Resolved{step, Stale, fmt.Sprintf("@%s passed, but at %s — the work has moved since", proof.Check, short(proof.SHA))}
	}
	if step.Ticked {
		if step.Section == Review && step.Check == "" {
			return Resolved{step, Attested, "a person ticked it; no command can check this one"}
		}
		return Resolved{step, Claimed, "ticked, but nothing has been run"}
	}
	return Resolved{step, Open, ""}
}

func matchingCommit(step Step, subjects []string) string {
	want := Normalise(step.Text)
	if len(want) < 12 {
		return ""
	}
	for _, subject := range subjects {
		got := Normalise(subject)
		if strings.Contains(got, want) || strings.Contains(want, got) {
			return subject
		}
	}
	return ""
}

type Report struct {
	Steps    []Resolved `json:"steps"`
	Proven   int        `json:"proven"`
	Attested int        `json:"attested"`
	Stale    int        `json:"stale"`
	Claimed  int        `json:"claimed"`
	Open     int        `json:"open"`
	Total    int        `json:"total"`
}

func Summarise(resolved []Resolved) Report {
	report := Report{Steps: resolved, Total: len(resolved)}
	for _, one := range resolved {
		switch one.State {
		case Proven:
			report.Proven++
		case Attested:
			report.Attested++
		case Stale:
			report.Stale++
		case Claimed:
			report.Claimed++
		case Open:
			report.Open++
		}
	}
	return report
}

func (r Report) Line() string {
	parts := []string{fmt.Sprintf("%d/%d proven", r.Proven, r.Total)}
	for _, part := range []struct {
		count int
		label string
	}{
		{r.Attested, "attested"},
		{r.Stale, "stale"},
		{r.Claimed, "claimed"},
		{r.Open, "open"},
	} {
		if part.count > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", part.count, part.label))
		}
	}
	return strings.Join(parts, " · ")
}

func short(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}
