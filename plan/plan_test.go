package plan_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/emusoi/mia-plugins/plan"
)

const sample = `# Due dates for tasks

## Implementation

- [x] Add the due_date column and backfill it @migrate
- [x] Sort the task list by due date
- [ ] the module still imports @lint

## Review

- [x] a second pair of eyes on the migration
- [ ] check the backfill against production row counts
`

func TestTickingEveryBoxProvesNothing(t *testing.T) {
	untouched := plan.Summarise(plan.Resolve(plan.Parse(sample), plan.World{Head: "abc123"}))

	everythingTicked := strings.ReplaceAll(sample, "- [ ]", "- [x]")
	ticked := plan.Summarise(plan.Resolve(plan.Parse(everythingTicked), plan.World{Head: "abc123"}))

	if ticked.Proven != untouched.Proven {
		t.Errorf("ticking every box changed proven from %d to %d", untouched.Proven, ticked.Proven)
	}
	if ticked.Proven != 0 {
		t.Errorf("%d steps are proven and nothing has been run", ticked.Proven)
	}
	if ticked.Claimed <= untouched.Claimed {
		t.Error("ticking boxes did not even register as a claim")
	}
}

func TestEvidenceProvesAndThenGoesStale(t *testing.T) {
	p := plan.Parse(sample)
	migrate := p.Steps[0]

	proofs := map[string]plan.Proof{
		migrate.Key(): {Key: migrate.Key(), Check: "migrate", Exit: 0, SHA: "abc123", At: time.Now()},
	}

	here := plan.Summarise(plan.Resolve(p, plan.World{Head: "abc123", Proofs: proofs}))
	if here.Proven != 1 {
		t.Fatalf("a passing check at this commit proved %d steps", here.Proven)
	}

	moved := plan.Resolve(p, plan.World{Head: "def456", Proofs: proofs})
	if moved[0].State != plan.Stale {
		t.Errorf("after a commit, a proven step is %q — a green tick that outlives what it tested is worse than no tick", moved[0].State)
	}
	if !strings.Contains(moved[0].Because, "moved") {
		t.Errorf("staleness did not explain itself: %q", moved[0].Because)
	}
}

func TestACommitNeverOverridesACheck(t *testing.T) {
	p := plan.Parse(sample)
	subjects := []string{
		"Sort the task list by due date",
		"Add the due_date column and backfill it",
		"the module still imports",
	}
	resolved := plan.Resolve(p, plan.World{Head: "abc123", Subjects: subjects})

	if resolved[1].State != plan.Proven {
		t.Errorf("a step naming no check was not proven by its commit: %q (%s)", resolved[1].State, resolved[1].Because)
	}
	for _, i := range []int{0, 2} {
		if resolved[i].State == plan.Proven {
			t.Errorf("a commit proved %q, which names a check — commits must never stand in for one", resolved[i].Step.Text)
		}
	}
}

func TestACommitNeverRescuesAFailingCheck(t *testing.T) {
	p := plan.Parse(sample)
	lint := p.Steps[2]
	resolved := plan.Resolve(p, plan.World{
		Head:     "abc123",
		Subjects: []string{"the module still imports"},
		Proofs: map[string]plan.Proof{
			lint.Key(): {Key: lint.Key(), Check: "lint", Exit: 1, SHA: "abc123"},
		},
	})
	if resolved[2].State != plan.Open {
		t.Errorf("a failing check resolved to %q", resolved[2].State)
	}
	if !strings.Contains(resolved[2].Because, "failed") {
		t.Errorf("the failure did not say it failed: %q", resolved[2].Because)
	}
}

func TestAReviewStepWithNoCheckIsOnlyEverAttested(t *testing.T) {
	resolved := plan.Resolve(plan.Parse(sample), plan.World{
		Head:     "abc123",
		Subjects: []string{"a second pair of eyes on the migration"},
	})
	eyes := resolved[3]
	if eyes.State != plan.Attested {
		t.Errorf("a ticked review step is %q, want attested", eyes.State)
	}
	if eyes.State == plan.Proven {
		t.Error("a commit proved a review step")
	}
	report := plan.Summarise(resolved)
	if !strings.Contains(report.Line(), "attested") {
		t.Errorf("the summary hides attestation: %q", report.Line())
	}
}

func TestEvidenceSurvivesReorderingAndNotRewriting(t *testing.T) {
	first := plan.Parse("- [ ] Sort the task list by due date\n- [ ] Add the column @migrate\n")
	reordered := plan.Parse("- [ ] Add the column @migrate\n- [ ]   sort   the Task list by due date  \n")
	if first.Steps[0].Key() != reordered.Steps[1].Key() {
		t.Error("moving a step and reformatting it lost its evidence")
	}

	rewritten := plan.Parse("- [ ] Sort the task list by due date, descending\n")
	if first.Steps[0].Key() == rewritten.Steps[0].Key() {
		t.Error("rewriting a step kept the proof of what it used to say")
	}
}

func TestTheLedgerKeepsEveryRunAndTheLatestWins(t *testing.T) {
	ledger := plan.Ledger{Path: filepath.Join(t.TempDir(), "evidence.jsonl")}
	for _, proof := range []plan.Proof{
		{Key: "k", Check: "lint", Exit: 1, SHA: "aaa"},
		{Key: "k", Check: "lint", Exit: 1, SHA: "bbb"},
		{Key: "k", Check: "lint", Exit: 0, SHA: "ccc"},
	} {
		if err := ledger.Append(proof); err != nil {
			t.Fatal(err)
		}
	}
	latest, err := ledger.Latest()
	if err != nil {
		t.Fatal(err)
	}
	if latest["k"].SHA != "ccc" || latest["k"].Exit != 0 {
		t.Errorf("the latest run is %+v", latest["k"])
	}
}

func TestTheStartersBlankItemsAreNotSteps(t *testing.T) {
	if steps := plan.Parse(plan.Starter("try/plancheck")).Steps; len(steps) != 0 {
		t.Errorf("an untouched starter plan has steps: %+v", steps)
	}
	if steps := plan.Parse("## Implementation\n\n- [ ] \n- [ ] add the flag\n").Steps; len(steps) != 1 || steps[0].Text != "add the flag" {
		t.Errorf("steps = %+v", steps)
	}
}
