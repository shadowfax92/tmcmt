package cmd

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"tmcmt/internal/targets"
	"tmcmt/internal/tmux"
)

func TestSelectionSendUsesRememberedLiveTargetsWithoutSelector(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := targets.Save("%1", []string{"%2", "%9"}); err != nil {
		t.Fatal(err)
	}

	restore := stubMulticastRuntime(t)
	defer restore()

	alivePaneIDs = func() (map[string]struct{}, error) {
		return map[string]struct{}{"%2": {}, "%3": {}}, nil
	}
	discoverMulticastCandidates = func(string, bool) ([]tmux.Candidate, error) {
		t.Fatal("discovery should not run when remembered live targets exist")
		return nil, nil
	}
	selectMulticastTargets = func([]tmux.Candidate, []string) ([]string, error) {
		t.Fatal("selector should not run when remembered live targets exist")
		return nil, nil
	}

	pasted := map[string]string{}
	pasteToPane = func(paneID, content string) error {
		pasted[paneID] = content
		return nil
	}

	if _, err := executeRootForTestWithInput(t, "raw selected text", "selection", "send", "--pane", "%1"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(mapsKeys(pasted), []string{"%2"}) {
		t.Fatalf("pasted panes = %#v, want only live remembered %%2", mapsKeys(pasted))
	}
	if pasted["%2"] != "raw selected text" {
		t.Fatalf("pasted content = %q", pasted["%2"])
	}
}

func TestSelectionSendSelectsStoresAndSendsWhenNoRememberedLiveTargets(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := targets.Save("%1", []string{"%9"}); err != nil {
		t.Fatal(err)
	}

	restore := stubMulticastRuntime(t)
	defer restore()

	alivePaneIDs = func() (map[string]struct{}, error) {
		return map[string]struct{}{"%2": {}, "%3": {}}, nil
	}
	discoverMulticastCandidates = func(source string, includeAll bool) ([]tmux.Candidate, error) {
		if source != "%1" || includeAll {
			t.Fatalf("discover called with source=%q includeAll=%v", source, includeAll)
		}
		return []tmux.Candidate{
			{Pane: tmux.Pane{ID: "%2"}, Tool: "codex"},
			{Pane: tmux.Pane{ID: "%3"}, Tool: "claude"},
		}, nil
	}
	selectMulticastTargets = func(_ []tmux.Candidate, remembered []string) ([]string, error) {
		if len(remembered) != 0 {
			t.Fatalf("remembered = %#v, want no live remembered targets", remembered)
		}
		return []string{"%2", "%3"}, nil
	}

	var pasted []string
	pasteToPane = func(paneID, content string) error {
		if content != "quick context" {
			t.Fatalf("content = %q", content)
		}
		pasted = append(pasted, paneID)
		return nil
	}

	if _, err := executeRootForTestWithInput(t, "quick context", "selection", "send", "--pane", "%1"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(pasted, []string{"%2", "%3"}) {
		t.Fatalf("pasted panes = %#v", pasted)
	}
	saved, err := targets.Load("%1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(saved, []string{"%2", "%3"}) {
		t.Fatalf("saved targets = %#v, want selected panes", saved)
	}
}

func TestSelectionSendCancelLeavesTargetsUntouched(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := targets.Save("%1", []string{"%9"}); err != nil {
		t.Fatal(err)
	}

	restore := stubMulticastRuntime(t)
	defer restore()

	alivePaneIDs = func() (map[string]struct{}, error) {
		return map[string]struct{}{"%2": {}}, nil
	}
	discoverMulticastCandidates = func(string, bool) ([]tmux.Candidate, error) {
		return []tmux.Candidate{{Pane: tmux.Pane{ID: "%2"}, Tool: "codex"}}, nil
	}
	selectMulticastTargets = func([]tmux.Candidate, []string) ([]string, error) {
		return nil, tmux.ErrSelectionCancelled
	}
	pasteToPane = func(string, string) error {
		t.Fatal("paste should not run when selector is cancelled")
		return nil
	}

	if _, err := executeRootForTestWithInput(t, "raw", "selection", "send", "--pane", "%1"); !errors.Is(err, ErrCancelled) {
		t.Fatalf("error = %v, want ErrCancelled", err)
	}
	saved, err := targets.Load("%1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(saved, []string{"%9"}) {
		t.Fatalf("saved targets = %#v, want unchanged %%9", saved)
	}
}

func TestSelectionTargetsUpdatesRememberedTargetsWithoutSending(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := targets.Save("%1", []string{"%3"}); err != nil {
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
	selectMulticastTargets = func(_ []tmux.Candidate, remembered []string) ([]string, error) {
		if !slices.Equal(remembered, []string{"%3"}) {
			t.Fatalf("remembered = %#v, want %%3", remembered)
		}
		return []string{"%2"}, nil
	}
	pasteToPane = func(string, string) error {
		t.Fatal("selection targets should not paste")
		return nil
	}

	if _, err := executeRootForTest(t, "selection", "targets", "--pane", "%1"); err != nil {
		t.Fatal(err)
	}
	saved, err := targets.Load("%1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(saved, []string{"%2"}) {
		t.Fatalf("saved targets = %#v, want %%2", saved)
	}
}

func TestSelectionSendRejectsEmptyStdinBeforeTargetResolution(t *testing.T) {
	restore := stubMulticastRuntime(t)
	defer restore()

	alivePaneIDs = func() (map[string]struct{}, error) {
		t.Fatal("target resolution should not run for empty stdin")
		return nil, nil
	}
	pasteToPane = func(string, string) error {
		t.Fatal("paste should not run for empty stdin")
		return nil
	}

	if _, err := executeRootForTestWithInput(t, "", "selection", "send", "--pane", "%1"); err == nil || !strings.Contains(err.Error(), "empty stdin") {
		t.Fatalf("error = %v, want empty stdin", err)
	}
}

func mapsKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
