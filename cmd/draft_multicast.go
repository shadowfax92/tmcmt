package cmd

import (
	"errors"
	"fmt"

	"tmcmt/internal/draft"
	"tmcmt/internal/targets"
	"tmcmt/internal/tmux"

	"github.com/spf13/cobra"
)

var (
	multicastPane     string
	multicastAllPanes bool
	multicastSend     bool

	discoverMulticastCandidates = tmux.DiscoverCodingPanes
	selectMulticastTargets      = tmux.SelectCandidates
	continueMulticast           = runMulticastDispatch
)

var multicastCmd = &cobra.Command{
	Use:   "multicast",
	Short: "Select coding panes and send the reviewed draft to them",
	RunE:  runDraftMulticast,
}

func init() {
	multicastCmd.Flags().StringVar(&multicastPane, "pane", "", "Source pane id (default: current pane)")
	multicastCmd.Flags().BoolVar(&multicastAllPanes, "all-panes", false, "Show all panes, not just detected coding panes")
	multicastCmd.Flags().BoolVar(&multicastSend, "send", false, "Press Enter after pasting into every target")
	draftCmd.AddCommand(multicastCmd)
}

// runDraftMulticast resolves target panes for a source draft, stores the
// selected targets, and hands off to the dispatch step.
func runDraftMulticast(cmd *cobra.Command, args []string) error {
	sourcePane, err := resolvePane(multicastPane)
	if err != nil {
		return err
	}
	if _, exists := draft.PathIfExists(sourcePane); !exists {
		_ = tmux.DisplayMessage(fmt.Sprintf("tmcmt: no draft for %s", sourcePane))
		return nil
	}

	candidates, err := discoverMulticastCandidates(sourcePane, multicastAllPanes)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		return errors.New("no target panes found")
	}

	live := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		live[candidate.Pane.ID] = struct{}{}
	}
	remembered, err := targets.Load(sourcePane, live)
	if err != nil {
		return err
	}
	targetIDs, err := selectMulticastTargets(candidates, remembered)
	if errors.Is(err, tmux.ErrSelectionCancelled) {
		return ErrCancelled
	}
	if err != nil {
		return err
	}
	if len(targetIDs) == 0 {
		return ErrCancelled
	}
	if err := targets.Save(sourcePane, targetIDs); err != nil {
		return err
	}
	return continueMulticast(sourcePane, targetIDs)
}

func runMulticastDispatch(sourcePane string, targetIDs []string) error {
	path, exists := draft.PathIfExists(sourcePane)
	if !exists {
		return fmt.Errorf("no draft for %s", sourcePane)
	}

	content, ok, err := reviewDraftFile(path, true)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	for _, paneID := range targetIDs {
		if !paneStillExists(paneID) {
			return fmt.Errorf("target pane %s no longer exists", paneID)
		}
	}
	for _, paneID := range targetIDs {
		if err := pasteToPane(paneID, content); err != nil {
			return fmt.Errorf("paste to %s: %w", paneID, err)
		}
	}
	if multicastSend {
		for _, paneID := range targetIDs {
			if err := sendEnterToPane(paneID); err != nil {
				return fmt.Errorf("send enter to %s: %w", paneID, err)
			}
		}
	}
	if _, err := archiveDraft(sourcePane); err != nil {
		return fmt.Errorf("archive draft: %w", err)
	}
	_ = tmux.DisplayMessage(fmt.Sprintf("tmcmt: draft sent to %d target%s and archived", len(targetIDs), plural(len(targetIDs))))
	return nil
}
