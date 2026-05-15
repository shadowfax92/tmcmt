package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

func TestMulticastReviewsAndPastesDraftToEveryTargetThenArchives(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := draft.Append("%1", "note", "selected"); err != nil {
		t.Fatal(err)
	}

	restore := stubMulticastRuntime(t)
	defer restore()

	discoverMulticastCandidates = func(string, bool) ([]tmux.Candidate, error) {
		return []tmux.Candidate{
			{Pane: tmux.Pane{ID: "%2"}, Tool: "codex"},
			{Pane: tmux.Pane{ID: "%3"}, Tool: "claude"},
		}, nil
	}
	selectMulticastTargets = func([]tmux.Candidate, []string) ([]string, error) {
		return []string{"%2", "%3"}, nil
	}

	var edited bool
	pasted := map[string]string{}
	editDraftInPopup = func(path string, insertMode bool) error {
		edited = true
		return nil
	}
	pasteToPane = func(paneID, content string) error {
		pasted[paneID] = content
		return nil
	}

	if _, err := executeRootForTest(t, "draft", "multicast", "--pane", "%1"); err != nil {
		t.Fatal(err)
	}
	if !edited {
		t.Fatal("expected editor review to run")
	}
	if pasted["%2"] == "" || pasted["%2"] != pasted["%3"] {
		t.Fatalf("pasted content = %#v", pasted)
	}
	if _, exists := draft.PathIfExists("%1"); exists {
		t.Fatal("expected source draft to be archived")
	}
	done := filepath.Join(home, ".local", "state", "tmcmt", "drafts", "done", "1-000001.md")
	if _, err := os.Stat(done); err != nil {
		t.Fatalf("expected archive %s: %v", done, err)
	}
}

func TestMulticastEmptyReviewClearsDraftAndSendsNothing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := draft.Append("%1", "note", "selected"); err != nil {
		t.Fatal(err)
	}

	restore := stubMulticastRuntime(t)
	defer restore()

	discoverMulticastCandidates = func(string, bool) ([]tmux.Candidate, error) {
		return []tmux.Candidate{{Pane: tmux.Pane{ID: "%2"}, Tool: "codex"}}, nil
	}
	selectMulticastTargets = func([]tmux.Candidate, []string) ([]string, error) {
		return []string{"%2"}, nil
	}
	editDraftInPopup = func(path string, insertMode bool) error {
		return os.WriteFile(path, []byte(" \n\t\n"), 0o644)
	}
	pasteToPane = func(string, string) error {
		t.Fatal("paste should not run for empty reviewed draft")
		return nil
	}

	if _, err := executeRootForTest(t, "draft", "multicast", "--pane", "%1"); err != nil {
		t.Fatal(err)
	}
	if _, exists := draft.PathIfExists("%1"); exists {
		t.Fatal("expected empty draft to be cleared")
	}
}

func TestMulticastStaleTargetLeavesDraftUnarchived(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := draft.Append("%1", "note", "selected"); err != nil {
		t.Fatal(err)
	}

	restore := stubMulticastRuntime(t)
	defer restore()

	discoverMulticastCandidates = func(string, bool) ([]tmux.Candidate, error) {
		return []tmux.Candidate{
			{Pane: tmux.Pane{ID: "%2"}, Tool: "codex"},
			{Pane: tmux.Pane{ID: "%3"}, Tool: "claude"},
		}, nil
	}
	selectMulticastTargets = func([]tmux.Candidate, []string) ([]string, error) {
		return []string{"%2", "%3"}, nil
	}
	paneStillExists = func(paneID string) bool {
		return paneID != "%3"
	}
	pasteToPane = func(string, string) error {
		t.Fatal("paste should not run when any target is stale")
		return nil
	}

	if _, err := executeRootForTest(t, "draft", "multicast", "--pane", "%1"); err == nil || !strings.Contains(err.Error(), "%3") {
		t.Fatalf("error = %v, want stale %%3 error", err)
	}
	if _, exists := draft.PathIfExists("%1"); !exists {
		t.Fatal("expected draft to remain active")
	}
}

func TestMulticastSendPressesEnterForEveryTarget(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := draft.Append("%1", "note", "selected"); err != nil {
		t.Fatal(err)
	}

	restore := stubMulticastRuntime(t)
	defer restore()

	discoverMulticastCandidates = func(string, bool) ([]tmux.Candidate, error) {
		return []tmux.Candidate{
			{Pane: tmux.Pane{ID: "%2"}, Tool: "codex"},
			{Pane: tmux.Pane{ID: "%3"}, Tool: "claude"},
		}, nil
	}
	selectMulticastTargets = func([]tmux.Candidate, []string) ([]string, error) {
		return []string{"%2", "%3"}, nil
	}

	var entered []string
	sendEnterToPane = func(paneID string) error {
		entered = append(entered, paneID)
		return nil
	}

	if _, err := executeRootForTest(t, "draft", "multicast", "--pane", "%1", "--send"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(entered, []string{"%2", "%3"}) {
		t.Fatalf("entered panes = %#v, want %%2 and %%3", entered)
	}
}

func stubMulticastDeps(t *testing.T) func() {
	t.Helper()
	origDiscover := discoverMulticastCandidates
	origSelect := selectMulticastTargets
	origContinue := continueMulticast
	origPane := multicastPane
	origAll := multicastAllPanes
	origSend := multicastSend

	multicastPane = ""
	multicastAllPanes = false
	multicastSend = false
	continueMulticast = func(string, []string) error { return nil }
	t.Cleanup(func() {
		multicastPane = origPane
		multicastAllPanes = origAll
		multicastSend = origSend
		resetMulticastFlags(t)
	})
	return func() {
		discoverMulticastCandidates = origDiscover
		selectMulticastTargets = origSelect
		continueMulticast = origContinue
	}
}

func stubMulticastRuntime(t *testing.T) func() {
	t.Helper()
	restoreDeps := stubMulticastDeps(t)
	origContinue := continueMulticast
	origEdit := editDraftInPopup
	origPaste := pasteToPane
	origEnter := sendEnterToPane
	origPaneExists := paneStillExists
	continueMulticast = runMulticastDispatch
	editDraftInPopup = func(string, bool) error { return nil }
	pasteToPane = func(string, string) error { return nil }
	sendEnterToPane = func(string) error { return nil }
	paneStillExists = func(string) bool { return true }
	return func() {
		restoreDeps()
		continueMulticast = origContinue
		editDraftInPopup = origEdit
		pasteToPane = origPaste
		sendEnterToPane = origEnter
		paneStillExists = origPaneExists
	}
}

func resetMulticastFlags(t *testing.T) {
	t.Helper()
	multicastPane = ""
	multicastAllPanes = false
	for _, name := range []string{"pane", "all-panes", "send"} {
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
