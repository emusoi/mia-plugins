package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/emusoi/mia-plugins/records"
)

func (p plugin) lessonVerb(args []string) int {
	asJSON := take(&args, "--json")
	sub, wt, rest, err := p.subject(args, "list", "edit", "open", "path")
	if err != nil {
		return failed(err)
	}
	name := strings.Join(rest, " ")
	dir := filepath.Join(p.miaDir, "lessons", p.key(wt.Branch))
	switch sub {
	case "list":
		names, err := p.lessons(wt.Branch)
		if err != nil {
			return failed(err)
		}
		if asJSON {
			return emit(names)
		}
		if len(names) == 0 {
			fmt.Printf("no lessons for %s yet — `mia lesson %s edit <name>` starts one\n", wt.Name, wt.Name)
		}
		for _, one := range names {
			fmt.Println(one)
		}
		return 0
	case "open":
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return failed(err)
		}
		return code(p.openEditor(dir))
	case "path":
		if name == "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return failed(err)
			}
			fmt.Println(dir)
			return 0
		}
		key, err := p.lessonKey(wt.Branch, name)
		if err != nil {
			return failed(err)
		}
		path := p.local().Path(records.Lessons, key)
		if _, err := os.Stat(path); err != nil {
			if err := p.local().Put(records.Lessons, key, []byte(lessonStarter(key))); err != nil {
				return failed(err)
			}
		}
		fmt.Println(path)
		return 0
	}
	key, err := p.lessonKey(wt.Branch, name)
	if err != nil {
		return failed(err)
	}
	return code(p.edit(records.Lessons, key, lessonStarter(key)))
}

func lessonStarter(key string) string {
	return "# " + key[strings.Index(key, "/")+1:] + "\n\n"
}

func (p plugin) lessonKey(branch, name string) (string, error) {
	name = strings.TrimSuffix(strings.TrimSpace(name), ".md")
	if name == "" {
		name = "notes"
	}
	if !filepath.IsLocal(name) {
		return "", fmt.Errorf("a lesson is named inside its branch's folder, and %q is not", name)
	}
	return p.key(branch) + "/" + name, nil
}

func (p plugin) lessons(branch string) ([]string, error) {
	keys, err := p.records(records.Lessons).List(records.Lessons, "")
	if err != nil {
		return nil, err
	}
	prefix := p.key(branch) + "/"
	var names []string
	for _, key := range keys {
		if after, found := strings.CutPrefix(key, prefix); found {
			names = append(names, after)
		}
	}
	sort.Strings(names)
	return names, nil
}
