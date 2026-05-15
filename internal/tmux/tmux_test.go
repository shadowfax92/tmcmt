package tmux

import (
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

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return sub == ""
}
