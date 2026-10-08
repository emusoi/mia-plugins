package plan

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Proof struct {
	Key    string    `json:"key"`
	Check  string    `json:"check"`
	Exit   int       `json:"exit"`
	SHA    string    `json:"sha"`
	At     time.Time `json:"at"`
	Output string    `json:"output,omitempty"`
	Text   string    `json:"text"`
	Dirty  bool      `json:"dirty,omitempty"`
}

type Ledger struct{ Path string }

func (l Ledger) Append(proof Proof) error {
	if err := os.MkdirAll(filepath.Dir(l.Path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(l.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	line, err := json.Marshal(proof)
	if err != nil {
		return err
	}
	_, err = file.Write(append(line, '\n'))
	return err
}

func LatestIn(lines []byte) map[string]Proof {
	latest := map[string]Proof{}
	for _, line := range strings.Split(string(lines), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var proof Proof
		if json.Unmarshal([]byte(line), &proof) != nil {
			continue
		}
		latest[proof.Key] = proof
	}
	return latest
}

func Line(proof Proof) ([]byte, error) {
	line, err := json.Marshal(proof)
	if err != nil {
		return nil, err
	}
	return append(line, '\n'), nil
}
func (l Ledger) Latest() (map[string]Proof, error) {
	file, err := os.Open(l.Path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]Proof{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	latest := map[string]Proof{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		var proof Proof
		if err := json.Unmarshal([]byte(text), &proof); err != nil {
			continue
		}
		latest[proof.Key] = proof
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", l.Path, err)
	}
	return latest, nil
}
