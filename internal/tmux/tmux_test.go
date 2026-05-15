package tmux

import (
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"tmcmt/internal/proc"
)

func TestParsePaneLineParsesTmuxMetadata(t *testing.T) {
	pane, err := parsePaneLine("$1\t%42\t123\tmain\twork\t2\t1\tvim\tagent\t/Users/me/project\tzsh")
	if err != nil {
		t.Fatal(err)
	}

	if pane.ID != "%42" || pane.PID != 123 || pane.SessionName != "main" ||
		pane.WindowIndex != "2" || pane.PaneIndex != "1" || pane.CWD != "/Users/me/project" ||
		pane.Command != "zsh" {
		t.Fatalf("pane parsed incorrectly: %#v", pane)
	}
}

func TestParsePaneLineRejectsMalformedRows(t *testing.T) {
	if _, err := parsePaneLine("too\tfew\tfields"); err == nil {
		t.Fatal("expected malformed row error")
	}
	if _, err := parsePaneLine("$1\t%42\tbad-pid\tmain\twork\t2\t1\ttitle\tlabel\t/tmp\tzsh"); err == nil {
		t.Fatal("expected bad pid error")
	}
}

func TestDiscoverCodingPanesMatchesDescendantsAndExcludesSource(t *testing.T) {
	panes := []Pane{
		{ID: "%1", PID: 100, SessionName: "main", WindowIndex: "1", WindowName: "src", PaneIndex: "0", CWD: "/repo", Command: "zsh"},
		{ID: "%2", PID: 200, SessionName: "main", WindowIndex: "2", WindowName: "codex", PaneIndex: "0", CWD: "/repo", Command: "zsh"},
		{ID: "%3", PID: 300, SessionName: "main", WindowIndex: "3", WindowName: "shell", PaneIndex: "0", CWD: "/repo", Command: "zsh"},
		{ID: "%4", PID: 400, SessionName: "main", WindowIndex: "4", WindowName: "claude", PaneIndex: "0", CWD: "/repo", Command: "claude"},
	}
	snap, err := proc.ParseSnapshot(`PID PPID ARGS
100 1 zsh
101 100 claude
200 1 zsh
201 200 /opt/homebrew/bin/codex --resume
300 1 zsh
301 300 sleep 1
400 1 claude
`)
	if err != nil {
		t.Fatal(err)
	}

	got := DiscoverCodingPanesFrom(panes, snap, "%1", false)
	if len(got) != 2 {
		t.Fatalf("candidate count = %d, want 2: %#v", len(got), got)
	}
	if got[0].Pane.ID != "%2" || got[0].Tool != "codex" {
		t.Fatalf("first candidate = %#v, want codex pane %%2", got[0])
	}
	if got[1].Pane.ID != "%4" || got[1].Tool != "claude" {
		t.Fatalf("second candidate = %#v, want claude pane %%4", got[1])
	}
}

func TestDiscoverCodingPanesCanIncludeAllPanes(t *testing.T) {
	panes := []Pane{
		{ID: "%1", PID: 100, SessionName: "main", WindowIndex: "1", WindowName: "src", PaneIndex: "0", CWD: "/repo", Command: "zsh"},
		{ID: "%2", PID: 200, SessionName: "main", WindowIndex: "2", WindowName: "shell", PaneIndex: "0", CWD: "/repo", Command: "zsh"},
	}
	snap, err := proc.ParseSnapshot("PID PPID ARGS\n100 1 zsh\n200 1 zsh\n")
	if err != nil {
		t.Fatal(err)
	}

	got := DiscoverCodingPanesFrom(panes, snap, "%1", true)
	if len(got) != 1 || got[0].Pane.ID != "%2" || got[0].Tool != "" {
		t.Fatalf("all-pane candidates = %#v, want non-source shell pane", got)
	}
}

func TestCandidateFormatIncludesToolContextAndPaneID(t *testing.T) {
	candidate := Candidate{
		Pane: Pane{
			ID:          "%7",
			SessionName: "main",
			WindowIndex: "3",
			WindowName:  "agents",
			PaneIndex:   "1",
			PaneLabel:   "codex-a",
			CWD:         "/Users/me/repo",
		},
		Tool: "codex",
	}

	row := candidate.Row()
	for _, part := range []string{"%7", "codex", "main", "3(agents).1(codex-a)", "/Users/me/repo"} {
		if !contains(row, part) {
			t.Fatalf("candidate row %q missing %q", row, part)
		}
	}
}

func TestCandidateRowsPutRememberedPanesFirst(t *testing.T) {
	candidates := []Candidate{
		{Pane: Pane{ID: "%2", SessionName: "main"}, Tool: "codex"},
		{Pane: Pane{ID: "%3", SessionName: "main"}, Tool: "claude"},
	}

	rows := candidateRows(candidates, map[string]struct{}{"%3": {}})
	if len(rows) != 2 || !strings.HasPrefix(rows[0], "%3\t") || !strings.HasPrefix(rows[1], "%2\t") {
		t.Fatalf("rows = %#v, want remembered pane first", rows)
	}
	if count := rememberedCandidateCount(candidates, map[string]struct{}{"%3": {}, "%9": {}}); count != 1 {
		t.Fatalf("remembered candidate count = %d, want 1", count)
	}
}

func TestFzfPreselectBindSelectsRememberedRowsFromTop(t *testing.T) {
	got := fzfPreselectBind(3)
	want := "start:first+select+down+select+down+select"
	if got != want {
		t.Fatalf("bind = %q, want %q", got, want)
	}
	if got := fzfPreselectBind(0); got != "" {
		t.Fatalf("zero bind = %q, want empty", got)
	}
}

func TestSelectCandidatesRequiresFzf(t *testing.T) {
	orig := lookPath
	lookPath = func(string) (string, error) {
		return "", os.ErrNotExist
	}
	t.Cleanup(func() {
		lookPath = orig
	})

	_, err := SelectCandidates([]Candidate{{Pane: Pane{ID: "%2"}, Tool: "codex"}}, nil)
	if err == nil || !contains(err.Error(), "multicast selection requires fzf") {
		t.Fatalf("error = %v, want fzf requirement", err)
	}
}

func TestSelectCandidatesTreatsEmptyCandidatesAsCancel(t *testing.T) {
	orig := lookPath
	lookPath = func(string) (string, error) {
		return "/opt/homebrew/bin/fzf", nil
	}
	t.Cleanup(func() {
		lookPath = orig
	})

	_, err := SelectCandidates(nil, nil)
	if !errors.Is(err, ErrSelectionCancelled) {
		t.Fatalf("error = %v, want selection cancelled", err)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return sub == ""
}

func TestSelectCandidatesRowsAcceptPaneIDColumn(t *testing.T) {
	rows := candidateRows([]Candidate{{Pane: Pane{ID: "%2", SessionName: "main"}, Tool: "codex"}}, nil)
	fields := splitTabs(rows[0])
	if !slices.Equal(fields[:2], []string{"%2", " "}) {
		t.Fatalf("row fields = %#v, want pane id then marker", fields)
	}
}

func splitTabs(s string) []string {
	var fields []string
	start := 0
	for i := range s {
		if s[i] == '\t' {
			fields = append(fields, s[start:i])
			start = i + 1
		}
	}
	return append(fields, s[start:])
}
