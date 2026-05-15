package proc

import "testing"

func TestParseSnapshotBuildsChildrenAndWalksDescendants(t *testing.T) {
	snap, err := ParseSnapshot(`  PID  PPID ARGS
  100     1 tmux: server
  200   100 -zsh
  201   200 codex
  202   200 sleep 1
  203   201 /bin/sh helper
`)
	if err != nil {
		t.Fatal(err)
	}

	var got []int
	snap.WalkDescendants(200, func(p Process) bool {
		got = append(got, p.PID)
		return false
	})
	want := []int{200, 201, 202, 203}
	if !equalInts(got, want) {
		t.Fatalf("descendants = %#v, want %#v", got, want)
	}
}

func TestWalkDescendantsCanStopEarly(t *testing.T) {
	snap, err := ParseSnapshot(`PID PPID ARGS
1 0 tmux
2 1 claude
3 2 helper
`)
	if err != nil {
		t.Fatal(err)
	}

	var got []int
	snap.WalkDescendants(1, func(p Process) bool {
		got = append(got, p.PID)
		return p.PID == 2
	})
	want := []int{1, 2}
	if !equalInts(got, want) {
		t.Fatalf("visited = %#v, want %#v", got, want)
	}
}

func TestExecutableNameUsesArgvZeroBasename(t *testing.T) {
	cases := map[string]string{
		"codex --resume":              "codex",
		"/opt/homebrew/bin/claude -p": "claude",
		"  cursor-agent":              "cursor-agent",
		"":                            "",
	}
	for args, want := range cases {
		if got := ExecutableName(args); got != want {
			t.Fatalf("ExecutableName(%q) = %q, want %q", args, got, want)
		}
	}
}

func TestParseSnapshotRejectsMalformedProcessRows(t *testing.T) {
	if _, err := ParseSnapshot("PID PPID ARGS\nnot-a-process"); err == nil {
		t.Fatal("expected malformed row error")
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
