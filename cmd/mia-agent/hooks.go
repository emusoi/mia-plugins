package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const hookCommand = "mia agent hook"

var claudeEvents = []string{"UserPromptSubmit", "PostToolUse", "Notification", "Stop", "SessionEnd"}

func splitThen(args []string) ([]string, []string, error) {
	for i, arg := range args {
		if arg != "--then" {
			continue
		}
		if i+1 >= len(args) {
			return nil, nil, errors.New("--then needs a JSON list: --then '[\"program\", \"arg\"]'")
		}
		var then []string
		if err := json.Unmarshal([]byte(args[i+1]), &then); err != nil || len(then) == 0 {
			return nil, nil, fmt.Errorf("--then %q is not a JSON list of a program and its arguments", args[i+1])
		}
		return append(append([]string{}, args[:i]...), args[i+2:]...), then, nil
	}
	return args, nil, nil
}

func runThen(then, rest []string) error {
	if len(then) == 0 {
		return nil
	}
	argv := append([]string{}, then[1:]...)
	if len(rest) > 1 {
		argv = append(argv, rest[len(rest)-1])
	}
	cmd := exec.Command(then[0], argv...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

func directHook(args []string) error {
	rest, then, err := splitThen(args)
	if err != nil {
		return err
	}
	record := exec.Command("mia", append([]string{"agent", "hook"}, rest...)...)
	record.Stdin = os.Stdin
	_ = record.Run()
	return runThen(then, rest)
}

func (p plugin) hook(args []string, stdin io.Reader) error {
	args, then, err := splitThen(args)
	if err != nil {
		return err
	}
	if err := p.record(args, stdin); err != nil {
		fmt.Fprintf(os.Stderr, "mia: %v\n", err)
	}
	return runThen(then, args)
}

func (p plugin) record(args []string, stdin io.Reader) error {
	if len(args) == 0 {
		return errors.New("usage: mia agent hook <claude|codex>")
	}
	var event struct {
		Name    string `json:"hook_event_name"`
		Message string `json:"message"`
		Type    string `json:"type"`
	}
	switch args[0] {
	case "claude":
		if err := json.NewDecoder(stdin).Decode(&event); err != nil {
			return fmt.Errorf("hook: %w", err)
		}
	case "codex":
		if len(args) < 2 {
			return nil
		}
		if err := json.Unmarshal([]byte(args[len(args)-1]), &event); err != nil {
			return fmt.Errorf("hook: %w", err)
		}
	default:
		return fmt.Errorf("no hook for %q", args[0])
	}
	state, forget := hookState(event.Name, event.Message, event.Type)
	if state == "" && !forget {
		return nil
	}
	if p.here == "" {
		return nil
	}
	window := paneWindow()
	if window == "" {
		return nil
	}
	remembered := p.recall()
	key := p.here + "\x00" + window
	if forget {
		delete(remembered, key)
	} else {
		remembered[key] = hooked(state)
	}
	p.remember(remembered)
	return nil
}

func hooked(state string) memory {
	if state == finished {
		return memory{Last: idle, Finished: true, Hooked: true}
	}
	return memory{Last: state, Hooked: true}
}

func hookState(name, message, kind string) (state string, forget bool) {
	switch name {
	case "UserPromptSubmit", "PostToolUse":
		return working, false
	case "Notification":
		if strings.Contains(strings.ToLower(message), "permission") {
			return waiting, false
		}
		return "", false
	case "Stop":
		return finished, false
	case "SessionEnd":
		return "", true
	}
	if kind == "agent-turn-complete" {
		return finished, false
	}
	return "", false
}

func paneWindow() string {
	pane := os.Getenv("TMUX_PANE")
	if pane == "" {
		return ""
	}
	out, err := exec.Command("tmux", "display-message", "-p", "-t", pane, "#{window_name}").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (p plugin) hooks(args []string) error {
	if len(args) != 1 || (args[0] != "install" && args[0] != "uninstall") {
		return errors.New("usage: mia agent hooks <install|uninstall>")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	install := args[0] == "install"
	path := filepath.Join(home, ".claude", "settings.json")
	if err := editClaude(path, install); err != nil {
		return err
	}
	fmt.Printf("claude: %s %s\n", map[bool]string{true: "hooks added to", false: "hooks removed from"}[install], path)
	if install {
		fmt.Printf("codex: add this line at the top of ~/.codex/config.toml\n  notify = [\"mia\", \"agent\", \"hook\", \"codex\"]\n")
	}
	return nil
}

func editClaude(path string, install bool) error {
	settings := map[string]any{}
	raw, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := json.Unmarshal(raw, &settings); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	case !errors.Is(err, os.ErrNotExist):
		return err
	}
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	for _, event := range claudeEvents {
		groups, _ := hooks[event].([]any)
		kept := groups[:0:0]
		for _, group := range groups {
			if !isOurs(group) {
				kept = append(kept, group)
			}
		}
		if install {
			kept = append(kept, map[string]any{
				"hooks": []any{map[string]any{"type": "command", "command": hookCommand + " claude 2>/dev/null || true"}},
			})
		}
		if len(kept) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = kept
		}
	}
	if len(hooks) == 0 {
		delete(settings, "hooks")
	} else {
		settings["hooks"] = hooks
	}
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0o644)
}

func isOurs(group any) bool {
	g, _ := group.(map[string]any)
	inner, _ := g["hooks"].([]any)
	for _, h := range inner {
		entry, _ := h.(map[string]any)
		if command, _ := entry["command"].(string); strings.HasPrefix(command, hookCommand) {
			return true
		}
	}
	return false
}
