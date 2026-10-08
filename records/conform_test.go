package records

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const shellHelper = `#!/bin/sh
# A store helper in the six verbs, as the contract documents it.
root="$MIA_STORE_ROOT"
verb=$1; kind=$2; key=$3
file="$root/$kind/$key"
case "$verb" in
  get)    [ -f "$file" ] || exit 2; cat "$file" ;;
  put)    mkdir -p "$root/$kind"; cat > "$file" ;;
  append) mkdir -p "$root/$kind"; cat >> "$file" ;;
  list)   [ -d "$root/$kind" ] || exit 0; ls -1 "$root/$kind" ;;
  open)   echo "opened $kind/$key" > "$root/opened" ;;
  url)    [ -f "$file" ] || exit 2; echo "https://example.test/$kind/$key" ;;
  *)      echo "unknown verb $verb" >&2; exit 1 ;;
esac
`

func helperOnPath(t *testing.T) (Helper, string) {
	t.Helper()
	bin := t.TempDir()
	root := t.TempDir()
	path := filepath.Join(bin, "mia-store-shelltest")
	if err := os.WriteFile(path, []byte(shellHelper), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("MIA_STORE_ROOT", root)
	helper, err := Find("shelltest")
	if err != nil {
		t.Fatal(err)
	}
	return helper, root
}

func TestAShellHelperSatisfiesTheWholeContract(t *testing.T) {
	helper, root := helperOnPath(t)

	if _, err := helper.Get(Plans, "absent"); !errors.Is(err, ErrAbsent) {
		t.Fatalf("exit 2 must mean ErrAbsent, got %v", err)
	}
	if err := helper.Put(Plans, "a-branch", []byte("# a plan\n")); err != nil {
		t.Fatal(err)
	}
	body, err := helper.Get(Plans, "a-branch")
	if err != nil || string(body) != "# a plan\n" {
		t.Fatalf("round trip = %q %v", body, err)
	}
	if err := helper.Append(Evidence, "a-branch", []byte("{\"one\":1}\n")); err != nil {
		t.Fatal(err)
	}
	if err := helper.Append(Evidence, "a-branch", []byte("{\"two\":2}\n")); err != nil {
		t.Fatal(err)
	}
	proofs, _ := helper.Get(Evidence, "a-branch")
	if strings.Count(string(proofs), "\n") != 2 {
		t.Errorf("append through a helper must add: %q", proofs)
	}
	keys, err := helper.List(Plans, "")
	if err != nil || len(keys) != 1 || keys[0] != "a-branch" {
		t.Fatalf("List = %v %v", keys, err)
	}
	url, err := helper.URL(Plans, "a-branch")
	if err != nil || url != "https://example.test/plans/a-branch" {
		t.Fatalf("URL = %q %v", url, err)
	}
	if err := helper.Open(Plans, "a-branch"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "opened")); err != nil {
		t.Errorf("open must reach the helper: %v", err)
	}
	if _, err := helper.URL(Plans, "absent"); !errors.Is(err, ErrAbsent) {
		t.Errorf("url for a missing record must be ErrAbsent, got %v", err)
	}
}

func TestAHelperFailureCarriesItsOwnStderr(t *testing.T) {
	helper, _ := helperOnPath(t)
	_, err := helper.run(nil, "nonsense", "plans", "x")
	if err == nil || !strings.Contains(err.Error(), "unknown verb nonsense") {
		t.Fatalf("err = %v, want the helper's own words", err)
	}
	if errors.Is(err, ErrAbsent) {
		t.Error("exit 1 is a failure, not an absent record")
	}
}

func TestAFallingStoreUsesAHelperUntilItBreaks(t *testing.T) {
	helper, root := helperOnPath(t)
	var warnings strings.Builder
	f := &Falling{To: helper, Local: Local{Dir: t.TempDir()}, Warn: &warnings}

	if err := f.Put(Plans, "while-up", []byte("through the helper")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "plans", "while-up")); err != nil {
		t.Fatalf("the helper should have written it: %v", err)
	}
	if warnings.Len() != 0 {
		t.Errorf("a working helper must not warn: %q", warnings.String())
	}

	os.Setenv("MIA_STORE_ROOT", "/proc/nonexistent/cannot-write")
	if err := f.Put(Plans, "while-down", []byte("fell to local")); err != nil {
		t.Fatalf("the fallback must succeed: %v", err)
	}
	if !strings.Contains(warnings.String(), "not answering") {
		t.Errorf("a broken helper must warn: %q", warnings.String())
	}
	body, err := f.Local.Get(Plans, "while-down")
	if err != nil || string(body) != "fell to local" {
		t.Fatalf("the local copy = %q %v", body, err)
	}
}
