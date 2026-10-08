package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"
	"time"
)

type rowInfo struct {
	Facts   []string `json:"facts,omitempty"`
	Status  string   `json:"status,omitempty"`
	Section string   `json:"section,omitempty"`
}

type rowsInput struct {
	Worktrees []struct {
		Name    string   `json:"name"`
		Session bool     `json:"session"`
		Windows []window `json:"windows,omitempty"`
	} `json:"worktrees"`
}

var urgency = map[string]int{waiting: 0, finished: 1, working: 2, idle: 3}

var looks = map[string]struct{ glyph, section string }{
	waiting:  {"●", "waiting"},
	finished: {"✓", "finished"},
	working:  {"◐", "working"},
	idle:     {"·", ""},
}

func (p plugin) rows(input rowsInput) map[string]rowInfo {
	answer := map[string]rowInfo{}
	table := loadProcs()
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, wt := range input.Worktrees {
		if !wt.Session {
			continue
		}
		var mine []window
		for _, w := range wt.Windows {
			if p.isAgentWindow(w, table) {
				mine = append(mine, w)
			}
		}
		if len(mine) == 0 {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			info, ok := summarise(p.agentsFrom(wt.Name, mine, table))
			if ok {
				mu.Lock()
				answer[wt.Name] = info
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return answer
}

func summarise(agents []agent) (rowInfo, bool) {
	if len(agents) == 0 {
		return rowInfo{}, false
	}
	sort.SliceStable(agents, func(i, j int) bool { return urgency[agents[i].State] < urgency[agents[j].State] })
	top := agents[0]
	look := looks[top.State]
	info := rowInfo{Section: look.section, Status: fmt.Sprintf("%s %s %s", look.glyph, top.State, top.Quiet)}
	if top.State == idle {
		info.Status = ""
	}
	if len(agents) > 1 {
		for _, a := range agents {
			info.Facts = append(info.Facts, fmt.Sprintf("%-8s%s %s %s", a.Window, looks[a.State].glyph, a.State, a.Quiet))
		}
	}
	return info, true
}

func (p plugin) rowsVerb(in io.Reader, out io.Writer) error {
	var input rowsInput
	if err := json.NewDecoder(in).Decode(&input); err != nil {
		return fmt.Errorf("rows: %w", err)
	}
	return json.NewEncoder(out).Encode(struct {
		Rows map[string]rowInfo `json:"rows"`
	}{p.rows(input)})
}

type rpc struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int            `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (p plugin) serve(in io.Reader, out io.Writer) error {
	var mu sync.Mutex
	send := func(message rpc) {
		message.JSONRPC = "2.0"
		data, err := json.Marshal(message)
		if err != nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		fmt.Fprintf(out, "%s\n", data)
	}
	go p.watch(func() { send(rpc{Method: "refresh"}) })
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 1<<20), 64<<20)
	for scanner.Scan() {
		var request rpc
		if json.Unmarshal(scanner.Bytes(), &request) != nil || request.ID == nil {
			continue
		}
		switch request.Method {
		case "rows":
			var input rowsInput
			if err := json.Unmarshal(request.Params, &input); err != nil {
				send(rpc{ID: request.ID, Error: &rpcError{Code: -32602, Message: err.Error()}})
				continue
			}
			send(rpc{ID: request.ID, Result: map[string]any{"rows": p.rows(input)}})
		case "panel":
			send(rpc{ID: request.ID, Result: p.everywherePanel()})
		default:
			send(rpc{ID: request.ID, Error: &rpcError{Code: -32601, Message: "no " + request.Method}})
		}
	}
	return scanner.Err()
}

func (p plugin) watch(changed func()) {
	last := stamp(p.statePath())
	for range time.Tick(500 * time.Millisecond) {
		if now := stamp(p.statePath()); now != last {
			last = now
			changed()
		}
	}
}

func stamp(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}
