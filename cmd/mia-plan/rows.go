package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/emusoi/mia-plugins/plan"
)

type rowInfo struct {
	Facts   []string `json:"facts,omitempty"`
	Status  string   `json:"status,omitempty"`
	Section string   `json:"section,omitempty"`
}

func (p plugin) rows(in io.Reader, out io.Writer) error {
	var input struct {
		Worktrees []worktree `json:"worktrees"`
	}
	if err := json.NewDecoder(in).Decode(&input); err != nil {
		return fmt.Errorf("rows: %w", err)
	}
	base := p.base()
	answer := map[string]rowInfo{}
	for _, wt := range input.Worktrees {
		if wt.Branch == "" {
			continue
		}
		pl, world, err := p.planFor(wt, base)
		if err != nil {
			continue
		}
		report := plan.Summarise(plan.Resolve(pl, world))
		if report.Total == 0 {
			continue
		}
		status := fmt.Sprintf("%d/%d proven", report.Proven, report.Total)
		section := "needs-proof"
		if report.Proven == report.Total {
			section = "proven"
		}
		answer[wt.Name] = rowInfo{Status: status, Section: section, Facts: []string{"plan    " + status}}
	}
	return json.NewEncoder(out).Encode(struct {
		Rows map[string]rowInfo `json:"rows"`
	}{answer})
}
