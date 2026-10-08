package plan

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

type Section string

const (
	Implementation Section = "Implementation"
	Review         Section = "Review"
)

type Step struct {
	Section Section `json:"section"`
	Text    string  `json:"text"`
	Check   string  `json:"check,omitempty"`
	Ticked  bool    `json:"ticked"`
	Line    int     `json:"line"`
}

func (s Step) Key() string { return Key(s.Text) }

func Key(text string) string {
	sum := sha256.Sum256([]byte(Normalise(text)))
	return hex.EncodeToString(sum[:])[:16]
}

var spaces = regexp.MustCompile(`\s+`)

func Normalise(text string) string {
	return spaces.ReplaceAllString(strings.ToLower(strings.TrimSpace(text)), " ")
}

type Shape string

const (
	Ship  Shape = "ship"
	Scout Shape = "scout"
)

type Plan struct {
	Title string `json:"title,omitempty"`
	Shape Shape  `json:"shape"`
	Steps []Step `json:"steps"`
}

func (p Plan) Delivers() string {
	if p.Shape == Scout {
		return "a report, not a change"
	}
	return "a landed change"
}

var (
	heading  = regexp.MustCompile(`^#{1,6}\s+(.*)$`)
	checkbox = regexp.MustCompile(`^\s*[-*]\s+\[([ xX])\]\s+(.*)$`)
	checkRef = regexp.MustCompile(`\s+@([a-zA-Z0-9][\w.-]*)\s*$`)
	shapeRef = regexp.MustCompile(`(?i)^\s*>\s*(ship|scout)\s*$`)
)

func Starter(branch string) string {
	return "# " + branch + "\n\n## Implementation\n\n- [ ] \n\n## Review\n\n- [ ] \n"
}

func Parse(markdown string) Plan {
	p := Plan{Shape: Ship}
	section := Implementation
	for number, line := range strings.Split(markdown, "\n") {
		if match := shapeRef.FindStringSubmatch(line); match != nil {
			p.Shape = Shape(strings.ToLower(match[1]))
			continue
		}
		if match := heading.FindStringSubmatch(line); match != nil {
			label := strings.TrimSpace(match[1])
			if p.Title == "" && strings.HasPrefix(line, "# ") {
				p.Title = label
				continue
			}
			switch strings.ToLower(label) {
			case "review":
				section = Review
			case "implementation":
				section = Implementation
			}
			continue
		}
		match := checkbox.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		text := strings.TrimSpace(match[2])
		check := ""
		if named := checkRef.FindStringSubmatch(text); named != nil {
			check = named[1]
			text = strings.TrimSpace(checkRef.ReplaceAllString(text, ""))
		}
		if text == "" && check == "" {
			continue
		}
		p.Steps = append(p.Steps, Step{
			Section: section,
			Text:    text,
			Check:   check,
			Ticked:  match[1] != " ",
			Line:    number + 1,
		})
	}
	return p
}
