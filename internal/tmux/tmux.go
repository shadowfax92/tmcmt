package tmux

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"tmcmt/internal/proc"
)

const paneFormat = "#{session_id}\t#{pane_id}\t#{pane_pid}\t#{session_name}\t#{window_name}\t#{window_index}\t#{pane_index}\t#{pane_title}\t#{@pane_label}\t#{?pane_current_path,#{pane_current_path},#{pane_start_path}}\t#{pane_current_command}"

var ErrSelectionCancelled = errors.New("selection cancelled")

var lookPath = exec.LookPath

type Pane struct {
	SessionID   string
	ID          string
	PID         int
	SessionName string
	WindowName  string
	WindowIndex string
	PaneIndex   string
	PaneTitle   string
	PaneLabel   string
	CWD         string
	Command     string
}

type Candidate struct {
	Pane     Pane
	Tool     string
	MatchPID int
}

type toolMatcher struct {
	tool      string
	exact     map[string]struct{}
	cmdRegexp *regexp.Regexp
}

var codingToolMatchers = []toolMatcher{
	exactTool("claude", "claude"),
	exactTool("codex", "codex"),
	regexTool("gemini", `\bgemini\b`),
	regexTool("qwen", `\bqwen\b`),
	regexTool("opencode", `\bopencode\b`),
	regexTool("aider", `\baider\b`),
	regexTool("cursor", `\bcursor-agent\b`),
	regexTool("crush", `\bcrush\b`),
	regexTool("grok", `\bgrok\b`),
	regexTool("copilot", `\bcopilot\b`),
	regexTool("pi", `\bpi\b`),
	regexTool("amazon-q", `\bqchat\b`),
}

func run(args ...string) (string, error) {
	cmd := exec.Command("tmux", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("tmux %s: %s (%w)",
			strings.Join(args, " "),
			strings.TrimSpace(string(out)),
			err)
	}
	return strings.TrimSpace(string(out)), nil
}

func IsInsideTmux() bool {
	return os.Getenv("TMUX") != ""
}

func CurrentPaneID() (string, error) {
	return run("display-message", "-p", "#{pane_id}")
}

// ListPanes returns tmux pane metadata for the entire server. The format stays
// tab-delimited so names and paths containing punctuation do not confuse parsing.
func ListPanes() ([]Pane, error) {
	out, err := run("list-panes", "-a", "-F", paneFormat)
	if err != nil {
		return nil, err
	}
	return parsePaneLines(out)
}

func PaneExists(paneID string) bool {
	_, err := run("display-message", "-t", paneID, "-p", "")
	return err == nil
}

// AlivePaneIDs returns the set of currently-alive pane ids ("%42") across
// the entire tmux server.
func AlivePaneIDs() (map[string]struct{}, error) {
	out, err := run("list-panes", "-a", "-F", "#{pane_id}")
	if err != nil {
		return nil, err
	}
	set := make(map[string]struct{})
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			set[line] = struct{}{}
		}
	}
	return set, nil
}

// DiscoverCodingPanes returns selector candidates for live coding-agent panes.
// It snapshots processes once, then matches pane descendant trees in Go.
func DiscoverCodingPanes(sourcePaneID string, includeAll bool) ([]Candidate, error) {
	panes, err := ListPanes()
	if err != nil {
		return nil, err
	}
	snap, err := proc.NewSnapshot()
	if err != nil {
		return nil, err
	}
	return DiscoverCodingPanesFrom(panes, snap, sourcePaneID, includeAll), nil
}

// DiscoverCodingPanesFrom filters pane metadata into selector candidates. It is
// pure so tests can exercise matching without requiring a live tmux server.
func DiscoverCodingPanesFrom(panes []Pane, snap *proc.Snapshot, sourcePaneID string, includeAll bool) []Candidate {
	if len(panes) == 0 {
		return nil
	}

	type result struct {
		index     int
		candidate Candidate
		ok        bool
	}
	workers := runtime.GOMAXPROCS(0)
	if workers < 1 {
		workers = 1
	}
	if workers > 8 {
		workers = 8
	}

	jobs := make(chan int)
	results := make(chan result, len(panes))
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				pane := panes[index]
				if pane.ID == sourcePaneID {
					results <- result{index: index}
					continue
				}
				tool, matchPID := matchPane(pane, snap)
				if tool == "" && !includeAll {
					results <- result{index: index}
					continue
				}
				results <- result{
					index: index,
					candidate: Candidate{
						Pane:     pane,
						Tool:     tool,
						MatchPID: matchPID,
					},
					ok: true,
				}
			}
		}()
	}
	for i := range panes {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	close(results)

	byIndex := make(map[int]Candidate)
	var indexes []int
	for r := range results {
		if !r.ok {
			continue
		}
		byIndex[r.index] = r.candidate
		indexes = append(indexes, r.index)
	}
	sort.Ints(indexes)

	out := make([]Candidate, 0, len(indexes))
	for _, index := range indexes {
		out = append(out, byIndex[index])
	}
	return out
}

// ClearSelection clears the copy-mode selection in the target pane without
// exiting copy mode.
func ClearSelection(paneID string) error {
	_, err := run("send-keys", "-t", paneID, "-X", "clear-selection")
	return err
}

// DisplayMessage shows a transient message in tmux's status line.
func DisplayMessage(msg string) error {
	_, err := run("display-message", msg)
	return err
}

// Row formats a selector line with the pane id first so fzf output can be
// mapped back to target panes without parsing human-oriented columns.
func (c Candidate) Row() string {
	tool := c.Tool
	if tool == "" {
		tool = "pane"
	}
	location := c.Pane.SessionName
	if part := tmuxPart(c.Pane.WindowIndex, c.Pane.WindowName); part != "" {
		location += " " + part
		if pane := tmuxPart(c.Pane.PaneIndex, c.Pane.PaneLabel); pane != "" {
			location += "." + pane
		}
	}
	return fmt.Sprintf("%s\t%-10s\t%s\t%s", c.Pane.ID, tool, location, c.Pane.CWD)
}

// SelectCandidates opens a tmux popup with fzf multi-select and returns the
// accepted pane ids. Remembered panes are shown first and marked in the list.
func SelectCandidates(candidates []Candidate, remembered []string) ([]string, error) {
	if _, err := lookPath("fzf"); err != nil {
		return nil, errors.New("multicast selection requires fzf")
	}
	if len(candidates) == 0 {
		return nil, ErrSelectionCancelled
	}

	rememberedSet := make(map[string]struct{}, len(remembered))
	for _, paneID := range remembered {
		rememberedSet[paneID] = struct{}{}
	}
	rows := candidateRows(candidates, rememberedSet)
	preselect := fzfPreselectBind(rememberedCandidateCount(candidates, rememberedSet))
	bindArg := ""
	if preselect != "" {
		bindArg = " --bind=" + shellQuote(preselect)
	}

	in, err := os.CreateTemp("", "tmcmt-targets-*.tsv")
	if err != nil {
		return nil, err
	}
	defer os.Remove(in.Name())
	if _, err := in.WriteString(strings.Join(rows, "\n") + "\n"); err != nil {
		_ = in.Close()
		return nil, err
	}
	if err := in.Close(); err != nil {
		return nil, err
	}

	out, err := os.CreateTemp("", "tmcmt-targets-selected-*.txt")
	if err != nil {
		return nil, err
	}
	outPath := out.Name()
	_ = out.Close()
	defer os.Remove(outPath)

	header := "Tab selects targets. Remembered targets are marked * and listed first."
	shellCmd := fmt.Sprintf(
		`fzf --multi --prompt=%s --header=%s --with-nth=2.. --accept-nth=1%s < %s > %s || true`,
		shellQuote("tmcmt targets> "),
		shellQuote(header),
		bindArg,
		shellQuote(in.Name()),
		shellQuote(outPath),
	)
	if _, err := run("display-popup", "-E", "-w", "90%", "-h", "80%", shellCmd); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		return nil, err
	}
	var selected []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			selected = append(selected, line)
		}
	}
	if len(selected) == 0 {
		return nil, ErrSelectionCancelled
	}
	return selected, nil
}

// SendEnter presses Enter in the target pane.
func SendEnter(paneID string) error {
	_, err := run("send-keys", "-t", paneID, "Enter")
	return err
}

// EditInPopup launches $EDITOR (or nvim) on the given file inside a tmux
// display-popup and blocks until the editor exits. If insertMode is true
// and the editor is nvim/vim, the editor starts in insert mode at line 1.
func EditInPopup(path string, insertMode bool) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "nvim"
	}

	var shellCmd string
	if insertMode && isVimLike(editor) {
		shellCmd = fmt.Sprintf(`TMCMT=1 %s -c 'normal! gg' -c 'startinsert' %s`,
			editor, shellQuote(path))
	} else {
		shellCmd = fmt.Sprintf(`TMCMT=1 %s %s`, editor, shellQuote(path))
	}

	_, err := run("display-popup", "-E", "-w", "80%", "-h", "80%", shellCmd)
	return err
}

func isVimLike(editor string) bool {
	e := strings.ToLower(editor)
	return strings.Contains(e, "nvim") || strings.Contains(e, "vim")
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// PasteToPane loads content into a tmux buffer and pastes it into the target
// pane using bracketed paste (-p). The buffer is deleted after the paste (-d)
// so it does not leak into the paste-buffer list.
func PasteToPane(paneID, content string) error {
	cmd := exec.Command("tmux", "load-buffer", "-b", "tmcmt", "-")
	cmd.Stdin = strings.NewReader(content)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("load-buffer: %s (%w)",
			strings.TrimSpace(string(out)), err)
	}
	if _, err := run("paste-buffer", "-b", "tmcmt", "-p", "-d", "-t", paneID); err != nil {
		return err
	}
	return nil
}

func parsePaneLines(output string) ([]Pane, error) {
	var panes []Pane
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		pane, err := parsePaneLine(line)
		if err != nil {
			return nil, err
		}
		panes = append(panes, pane)
	}
	return panes, nil
}

func parsePaneLine(line string) (Pane, error) {
	parts := strings.Split(line, "\t")
	if len(parts) != 11 {
		return Pane{}, fmt.Errorf("malformed tmux pane row %q", line)
	}
	pid, err := strconv.Atoi(parts[2])
	if err != nil {
		return Pane{}, fmt.Errorf("parse pane pid in %q: %w", line, err)
	}
	return Pane{
		SessionID:   parts[0],
		ID:          parts[1],
		PID:         pid,
		SessionName: parts[3],
		WindowName:  parts[4],
		WindowIndex: parts[5],
		PaneIndex:   parts[6],
		PaneTitle:   parts[7],
		PaneLabel:   parts[8],
		CWD:         parts[9],
		Command:     parts[10],
	}, nil
}

func matchPane(pane Pane, snap *proc.Snapshot) (string, int) {
	if tool := matchArgs(pane.Command); tool != "" {
		return tool, pane.PID
	}
	if snap == nil {
		return "", 0
	}
	var tool string
	var matchPID int
	snap.WalkDescendants(pane.PID, func(p proc.Process) bool {
		tool = matchArgs(p.Args)
		if tool == "" {
			return false
		}
		matchPID = p.PID
		return true
	})
	return tool, matchPID
}

func matchArgs(args string) string {
	exe := proc.ExecutableName(args)
	for _, matcher := range codingToolMatchers {
		if _, ok := matcher.exact[exe]; ok {
			return matcher.tool
		}
		if matcher.cmdRegexp != nil && matcher.cmdRegexp.MatchString(args) {
			return matcher.tool
		}
	}
	return ""
}

func exactTool(tool string, names ...string) toolMatcher {
	exact := make(map[string]struct{}, len(names))
	for _, name := range names {
		exact[name] = struct{}{}
	}
	return toolMatcher{tool: tool, exact: exact}
}

func regexTool(tool, pattern string) toolMatcher {
	return toolMatcher{tool: tool, exact: map[string]struct{}{}, cmdRegexp: regexp.MustCompile(pattern)}
}

func tmuxPart(index, label string) string {
	if index == "" {
		return ""
	}
	if label != "" {
		return fmt.Sprintf("%s(%s)", index, label)
	}
	return index
}

func candidateRows(candidates []Candidate, remembered map[string]struct{}) []string {
	ordered := make([]Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		if _, ok := remembered[candidate.Pane.ID]; ok {
			ordered = append(ordered, candidate)
		}
	}
	for _, candidate := range candidates {
		if _, ok := remembered[candidate.Pane.ID]; !ok {
			ordered = append(ordered, candidate)
		}
	}

	rows := make([]string, 0, len(ordered))
	for _, candidate := range ordered {
		row := candidate.Row()
		if _, ok := remembered[candidate.Pane.ID]; ok {
			row = strings.Replace(row, "\t", "\t*\t", 1)
		} else {
			row = strings.Replace(row, "\t", "\t \t", 1)
		}
		rows = append(rows, row)
	}
	return rows
}

func rememberedCandidateCount(candidates []Candidate, remembered map[string]struct{}) int {
	count := 0
	for _, candidate := range candidates {
		if _, ok := remembered[candidate.Pane.ID]; ok {
			count++
		}
	}
	return count
}

func fzfPreselectBind(count int) string {
	if count <= 0 {
		return ""
	}
	actions := []string{"first"}
	for i := 0; i < count; i++ {
		actions = append(actions, "select")
		if i < count-1 {
			actions = append(actions, "down")
		}
	}
	return "start:" + strings.Join(actions, "+")
}
