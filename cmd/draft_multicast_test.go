package cmd

import (
	"errors"
	"slices"
	"testing"

	"tmcmt/internal/draft"
	"tmcmt/internal/targets"
	"tmcmt/internal/tmux"
)

func TestMulticastSelectsCodingPanesAndStoresTargetsBeforeDispatch(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := draft.Append("%1", "note", "selected"); err != nil {
		t.Fatal(err)
	}

	var selectedCandidates []tmux.Candidate
	var selectedRemembered []string
	var dispatched []string
	restore := stubMulticastDeps(t)
	defer restore()

	discoverMulticastCandidates = func(source string, includeAll bool) ([]tmux.Candidate, error) {
		if source != "%1" || includeAll {
			t.Fatalf("discover called with source=%q includeAll=%v", source, includeAll)
		}
		return []tmux.Candidate{
			{Pane: tmux.Pane{ID: "%2"}, Tool: "codex"},
			{Pane: tmux.Pane{ID: "%3"}, Tool: "claude"},
		}, nil
	}
	selectMulticastTargets = func(candidates []tmux.Candidate, remembered []string) ([]string, error) {
		selectedCandidates = candidates
		selectedRemembered = remembered
		return []string{"%2", "%3"}, nil
	}
	continueMulticast = func(source string, targetIDs []string) error {
		dispatched = append([]string(nil), targetIDs...)
		saved, err := targets.Load(source, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(saved, targetIDs) {
			t.Fatalf("saved targets before dispatch = %#v, want %#v", saved, targetIDs)
		}
		return nil
	}

	if _, err := executeRootForTest(t, "draft", "multicast", "--pane", "%1"); err != nil {
		t.Fatal(err)
	}
	if len(selectedCandidates) != 2 {
		t.Fatalf("selector candidate count = %d, want 2", len(selectedCandidates))
	}
	if len(selectedRemembered) != 0 {
		t.Fatalf("remembered targets = %#v, want empty", selectedRemembered)
	}
	if !slices.Equal(dispatched, []string{"%2", "%3"}) {
		t.Fatalf("dispatched targets = %#v", dispatched)
	}
}

func TestMulticastSelectorReceivesLiveRememberedTargets(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := draft.Append("%1", "note", "selected"); err != nil {
		t.Fatal(err)
	}
	if err := targets.Save("%1", []string{"%3", "%9"}); err != nil {
		t.Fatal(err)
	}

	restore := stubMulticastDeps(t)
	defer restore()

	discoverMulticastCandidates = func(string, bool) ([]tmux.Candidate, error) {
		return []tmux.Candidate{
			{Pane: tmux.Pane{ID: "%2"}, Tool: "codex"},
			{Pane: tmux.Pane{ID: "%3"}, Tool: "claude"},
		}, nil
	}
	selectMulticastTargets = func(_ []tmux.Candidate, remembered []string) ([]string, error) {
		if !slices.Equal(remembered, []string{"%3"}) {
			t.Fatalf("remembered = %#v, want only live %%3", remembered)
		}
		return []string{"%3"}, nil
	}

	if _, err := executeRootForTest(t, "draft", "multicast", "--pane", "%1"); err != nil {
		t.Fatal(err)
	}
}

func TestMulticastCancelLeavesDraftAndRememberedTargetsUntouched(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := draft.Append("%1", "note", "selected"); err != nil {
		t.Fatal(err)
	}
	if err := targets.Save("%1", []string{"%3"}); err != nil {
		t.Fatal(err)
	}

	restore := stubMulticastDeps(t)
	defer restore()

	discoverMulticastCandidates = func(string, bool) ([]tmux.Candidate, error) {
		return []tmux.Candidate{{Pane: tmux.Pane{ID: "%2"}, Tool: "codex"}}, nil
	}
	selectMulticastTargets = func([]tmux.Candidate, []string) ([]string, error) {
		return nil, ErrCancelled
	}
	continueMulticast = func(string, []string) error {
		t.Fatal("dispatch should not run on cancel")
		return nil
	}

	if _, err := executeRootForTest(t, "draft", "multicast", "--pane", "%1"); !errors.Is(err, ErrCancelled) {
		t.Fatalf("error = %v, want ErrCancelled", err)
	}
	if _, exists := draft.PathIfExists("%1"); !exists {
		t.Fatal("draft was removed after selector cancel")
	}
	got, err := targets.Load("%1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"%3"}) {
		t.Fatalf("remembered targets = %#v, want unchanged %%3", got)
	}
}

func stubMulticastDeps(t *testing.T) func() {
	t.Helper()
	origDiscover := discoverMulticastCandidates
	origSelect := selectMulticastTargets
	origContinue := continueMulticast
	origPane := multicastPane
	origAll := multicastAllPanes

	multicastPane = ""
	multicastAllPanes = false
	continueMulticast = func(string, []string) error { return nil }
	t.Cleanup(func() {
		multicastPane = origPane
		multicastAllPanes = origAll
		resetMulticastFlags(t)
	})
	return func() {
		discoverMulticastCandidates = origDiscover
		selectMulticastTargets = origSelect
		continueMulticast = origContinue
	}
}

func resetMulticastFlags(t *testing.T) {
	t.Helper()
	multicastPane = ""
	multicastAllPanes = false
	for _, name := range []string{"pane", "all-panes"} {
		flag := multicastCmd.Flags().Lookup(name)
		if flag == nil {
			t.Fatalf("multicast flag %s missing", name)
		}
		if err := flag.Value.Set(flag.DefValue); err != nil {
			t.Fatal(err)
		}
		flag.Changed = false
	}
}
