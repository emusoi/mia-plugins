package plan

import "testing"

func TestAPlanShipsUnlessItSaysOtherwise(t *testing.T) {
	p := Parse("# a branch\n\n## Implementation\n- [ ] do the thing @test\n")
	if p.Shape != Ship {
		t.Fatalf("shape = %q, want ship by default", p.Shape)
	}
	if p.Delivers() != "a landed change" {
		t.Errorf("Delivers = %q", p.Delivers())
	}
}

func TestAScoutPlanSaysSoUnderTheTitle(t *testing.T) {
	p := Parse("# why is login flaky\n\n> scout\n\n## Implementation\n- [ ] read the logs @test\n")
	if p.Shape != Scout {
		t.Fatalf("shape = %q, want scout", p.Shape)
	}
	if p.Delivers() != "a report, not a change" {
		t.Errorf("Delivers = %q", p.Delivers())
	}
	if len(p.Steps) != 1 {
		t.Errorf("the shape line must not be read as a step: %+v", p.Steps)
	}
	if p.Title != "why is login flaky" {
		t.Errorf("title = %q", p.Title)
	}
}

func TestTheShapeLineIsCaseInsensitiveAndAnywhere(t *testing.T) {
	for _, body := range []string{
		"# t\n> Scout\n## Implementation\n- [ ] x @t\n",
		"# t\n## Implementation\n>  SCOUT  \n- [ ] x @t\n",
	} {
		if got := Parse(body).Shape; got != Scout {
			t.Errorf("Parse(%q).Shape = %q", body, got)
		}
	}
}

func TestAnOrdinaryQuoteIsNotAShape(t *testing.T) {
	p := Parse("# t\n\n> we decided against the cache\n\n## Implementation\n- [ ] x @t\n")
	if p.Shape != Ship {
		t.Fatalf("a normal blockquote must not change the shape: %q", p.Shape)
	}
}
