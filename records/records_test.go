package records

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalRoundTripsEachKindAtItsOwnPath(t *testing.T) {
	l := Local{Dir: t.TempDir()}
	if _, err := l.Get(Plans, "nothing"); !errors.Is(err, ErrAbsent) {
		t.Fatalf("a missing record must be ErrAbsent, got %v", err)
	}
	if err := l.Put(Plans, "a-branch", []byte("# a plan\n")); err != nil {
		t.Fatal(err)
	}
	body, err := l.Get(Plans, "a-branch")
	if err != nil || string(body) != "# a plan\n" {
		t.Fatalf("Get = %q %v", body, err)
	}
	if _, err := os.Stat(filepath.Join(l.Dir, "plans", "a-branch.md")); err != nil {
		t.Errorf("a plan must land at plans/<key>.md: %v", err)
	}
	if err := l.Append(Evidence, "a-branch", []byte("{\"one\":1}\n")); err != nil {
		t.Fatal(err)
	}
	if err := l.Append(Evidence, "a-branch", []byte("{\"two\":2}\n")); err != nil {
		t.Fatal(err)
	}
	proofs, _ := l.Get(Evidence, "a-branch")
	if strings.Count(string(proofs), "\n") != 2 {
		t.Errorf("append must add, not replace: %q", proofs)
	}
	keys, _ := l.List(Plans, "")
	if len(keys) != 1 || keys[0] != "a-branch" {
		t.Errorf("List = %v, want the key without its extension", keys)
	}
}

type flaky struct {
	Local
	fail bool
	puts int
}

func (f *flaky) Name() string { return "flaky" }
func (f *flaky) Get(k Kind, key string) ([]byte, error) {
	if f.fail {
		return nil, errors.New("not answering")
	}
	return f.Local.Get(k, key)
}
func (f *flaky) Put(k Kind, key string, b []byte) error {
	if f.fail {
		return errors.New("not answering")
	}
	f.puts++
	return f.Local.Put(k, key, b)
}

func TestAnUnreachableBackendFallsToLocalAndWarnsOnce(t *testing.T) {
	backend := &flaky{Local: Local{Dir: t.TempDir()}, fail: true}
	var warnings strings.Builder
	f := &Falling{To: backend, Local: Local{Dir: t.TempDir()}, Warn: &warnings}

	if err := f.Put(Plans, "one", []byte("first")); err != nil {
		t.Fatalf("a fallback put must succeed: %v", err)
	}
	if err := f.Put(Plans, "two", []byte("second")); err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(warnings.String(), "not answering"); got != 1 {
		t.Errorf("warned %d times, want once per kind", got)
	}
	body, err := f.Get(Plans, "one")
	if err != nil || string(body) != "first" {
		t.Fatalf("the local copy must be readable: %q %v", body, err)
	}
}

func TestAnAbsentRecordIsNotAnOutage(t *testing.T) {
	backend := &flaky{Local: Local{Dir: t.TempDir()}}
	var warnings strings.Builder
	f := &Falling{To: backend, Local: Local{Dir: t.TempDir()}, Warn: &warnings}
	if _, err := f.Get(Plans, "never-written"); !errors.Is(err, ErrAbsent) {
		t.Fatalf("Get = %v, want ErrAbsent passed through", err)
	}
	if warnings.Len() != 0 {
		t.Errorf("a missing record must not look like an outage: %q", warnings.String())
	}
}

func TestAHelperThatIsNotOnPathSaysWhich(t *testing.T) {
	_, err := Find("definitely-not-installed")
	if err == nil || !strings.Contains(err.Error(), "mia-store-definitely-not-installed") {
		t.Fatalf("err = %v, want the tool name it looked for", err)
	}
}

func TestALocalKeyCannotReachOutsideTheStore(t *testing.T) {
	dir := t.TempDir()
	store := Local{Dir: filepath.Join(dir, "mia")}
	for _, key := range []string{"../../escaped", "/etc/escaped", "a/../../escaped"} {
		if err := store.Put(Lessons, key, []byte("x")); err == nil {
			t.Errorf("Put %q was allowed", key)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "escaped.md")); err == nil {
		t.Error("a key wrote outside the store")
	}
	if _, err := store.List(Issues, "../.."); err == nil {
		t.Error("a listing reached outside the store")
	}
	if err := store.Put(Lessons, "branch/topic/notes", []byte("x")); err != nil {
		t.Errorf("a nested key inside the store was refused: %v", err)
	}
}
