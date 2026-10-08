package records

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type Kind string

const (
	Plans    Kind = "plans"
	Evidence Kind = "evidence"
	Lessons  Kind = "lessons"
	Issues   Kind = "issues"
)

func Kinds() []Kind { return []Kind{Plans, Evidence, Lessons, Issues} }

var ErrAbsent = errors.New("no such record")

type Store interface {
	Name() string
	Get(kind Kind, key string) ([]byte, error)
	Put(kind Kind, key string, body []byte) error
	Append(kind Kind, key string, body []byte) error
	List(kind Kind, under string) ([]string, error)
	Open(kind Kind, key string) error
	URL(kind Kind, key string) (string, error)
}

type Local struct{ Dir string }

func (l Local) Name() string { return "local" }

func (l Local) Path(kind Kind, key string) string { return l.path(kind, key) }

func (l Local) path(kind Kind, key string) string {
	switch kind {
	case Plans:
		return filepath.Join(l.Dir, "plans", key+".md")
	case Evidence:
		return filepath.Join(l.Dir, "evidence", key+".jsonl")
	case Lessons:
		return filepath.Join(l.Dir, "lessons", key+".md")
	default:
		return filepath.Join(l.Dir, string(kind), key+".md")
	}
}

func (l Local) checked(kind Kind, key string) (string, error) {
	if !filepath.IsLocal(key) {
		return "", fmt.Errorf("%s key %q reaches outside %s", kind, key, l.Dir)
	}
	return l.path(kind, key), nil
}

func (l Local) Get(kind Kind, key string) ([]byte, error) {
	path, err := l.checked(kind, key)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrAbsent
	}
	return data, err
}

func (l Local) Put(kind Kind, key string, body []byte) error {
	path, err := l.checked(kind, key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}

func (l Local) Append(kind Kind, key string, body []byte) error {
	path, err := l.checked(kind, key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(body)
	return err
}

func (l Local) List(kind Kind, under string) ([]string, error) {
	root := filepath.Dir(l.path(kind, "x"))
	if kind == Lessons {
		root = filepath.Join(l.Dir, "lessons")
	}
	if under != "" {
		if !filepath.IsLocal(under) {
			return nil, fmt.Errorf("%s under %q reaches outside %s", kind, under, l.Dir)
		}
		root = filepath.Join(root, under)
	}
	var keys []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = strings.TrimSuffix(strings.TrimSuffix(rel, ".md"), ".jsonl")
		keys = append(keys, filepath.ToSlash(rel))
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sort.Strings(keys)
	return keys, nil
}

func (l Local) Open(kind Kind, key string) error {
	return fmt.Errorf("the local store has no application to open — the file is %s", l.path(kind, key))
}

func (l Local) URL(kind Kind, key string) (string, error) { return "", ErrAbsent }

type Helper struct {
	Tool string
	name string
}

const missing = 2

func Find(name string) (Helper, error) {
	tool := "mia-store-" + name
	path, err := exec.LookPath(tool)
	if err != nil {
		return Helper{}, fmt.Errorf("no store called %q — %s is not on PATH", name, tool)
	}
	return Helper{Tool: path, name: name}, nil
}

func (h Helper) Name() string { return h.name }

func (h Helper) run(stdin []byte, args ...string) ([]byte, error) {
	command := exec.Command(h.Tool, args...)
	if stdin != nil {
		command.Stdin = strings.NewReader(string(stdin))
	}
	var stderr strings.Builder
	command.Stderr = &stderr
	out, err := command.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == missing {
			return nil, ErrAbsent
		}
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return nil, fmt.Errorf("%s %s: %s", filepath.Base(h.Tool), strings.Join(args, " "), detail)
	}
	return out, nil
}

func (h Helper) Get(kind Kind, key string) ([]byte, error) {
	return h.run(nil, "get", string(kind), key)
}

func (h Helper) Put(kind Kind, key string, body []byte) error {
	_, err := h.run(body, "put", string(kind), key)
	return err
}

func (h Helper) Append(kind Kind, key string, body []byte) error {
	_, err := h.run(body, "append", string(kind), key)
	return err
}

func (h Helper) List(kind Kind, under string) ([]string, error) {
	args := []string{"list", string(kind)}
	if under != "" {
		args = append(args, under)
	}
	out, err := h.run(nil, args...)
	if err != nil {
		return nil, err
	}
	var keys []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			keys = append(keys, line)
		}
	}
	return keys, nil
}

func (h Helper) Open(kind Kind, key string) error {
	_, err := h.run(nil, "open", string(kind), key)
	return err
}

func (h Helper) URL(kind Kind, key string) (string, error) {
	out, err := h.run(nil, "url", string(kind), key)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
