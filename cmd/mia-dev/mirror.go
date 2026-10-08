package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

func mirror(from, to string) (int, error) {
	want, err := files(from)
	if err != nil {
		return 0, err
	}
	have, err := files(to)
	if err != nil {
		return 0, err
	}
	changed := 0
	for rel := range want {
		did, err := copyIfDifferent(filepath.Join(from, rel), filepath.Join(to, rel))
		if err != nil {
			return changed, err
		}
		if did {
			changed++
		}
	}
	for rel := range have {
		if want[rel] {
			continue
		}
		target := filepath.Join(to, rel)
		if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
			return changed, err
		}
		changed++
		removeEmptyParents(filepath.Dir(target), to)
	}
	return changed, nil
}

func files(dir string) (map[string]bool, error) {
	out, err := git(dir, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	found := map[string]bool{}
	for _, rel := range strings.Split(out, "\x00") {
		if rel == "" {
			continue
		}
		if _, err := os.Lstat(filepath.Join(dir, rel)); err == nil {
			found[rel] = true
		}
	}
	return found, nil
}

func copyIfDifferent(source, target string) (bool, error) {
	info, err := os.Lstat(source)
	if err != nil {
		return false, err
	}
	if existing, err := os.Lstat(target); err == nil && existing.IsDir() {
		if err := os.RemoveAll(target); err != nil {
			return false, err
		}
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return false, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		link, err := os.Readlink(source)
		if err != nil {
			return false, err
		}
		if current, err := os.Readlink(target); err == nil && current == link {
			return false, nil
		}
		os.Remove(target)
		return true, os.Symlink(link, target)
	}
	body, err := os.ReadFile(source)
	if err != nil {
		return false, err
	}
	if current, err := os.Lstat(target); err == nil && current.Mode().IsRegular() && current.Mode().Perm() == info.Mode().Perm() {
		if existing, err := os.ReadFile(target); err == nil && bytes.Equal(existing, body) {
			return false, nil
		}
	}
	if err := os.WriteFile(target, body, info.Mode().Perm()); err != nil {
		return false, err
	}
	return true, os.Chmod(target, info.Mode().Perm())
}

func removeEmptyParents(dir, root string) {
	for dir != root && os.Remove(dir) == nil {
		dir = filepath.Dir(dir)
	}
}
