package plan

import "testing"

func TestAStepThatBecameALayerTakesItsStateFromTheLayer(t *testing.T) {
	p := Parse("- [ ] schema @migrate\n- [ ] api endpoint\n- [ ] docs\n")
	world := World{Head: "abc", Layers: map[string]LayerFact{
		p.Steps[0].Key(): {Branch: "schema", Landed: true},
		p.Steps[1].Key(): {Branch: "api-endpoint", Proven: 2, Total: 2},
		p.Steps[2].Key(): {Branch: "docs", Proven: 1, Total: 3},
	}}
	got := Resolve(p, world)
	if got[0].State != Proven || got[1].State != Attested || got[2].State != Open {
		t.Errorf("states %v %v %v", got[0].State, got[1].State, got[2].State)
	}
	if got[2].Because != "layer docs: 1/3 proven" {
		t.Errorf("because %q", got[2].Because)
	}
}

func TestAPassOnUncommittedChangesProvesNoCommit(t *testing.T) {
	p := Parse("- [ ] it builds @ok\n")
	key := p.Steps[0].Key()
	clean := Resolve(p, World{Head: "abc", Proofs: map[string]Proof{key: {Key: key, Check: "ok", SHA: "abc"}}})
	dirty := Resolve(p, World{Head: "abc", Proofs: map[string]Proof{key: {Key: key, Check: "ok", SHA: "abc", Dirty: true}}})
	if clean[0].State != Proven {
		t.Errorf("a clean pass at the head is %s", clean[0].State)
	}
	if dirty[0].State == Proven {
		t.Errorf("a pass on uncommitted changes counts as proof of the commit: %+v", dirty[0])
	}
}
