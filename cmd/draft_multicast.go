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

	discoverMulticastCandidates = tmux.DiscoverCodingPanes
	selectMulticastTargets      = tmux.SelectCandidates
	continueMulticast           = continueMulticastStub
)

var multicastCmd = &cobra.Command{
	Use:   "multicast",
	Short: "Select coding panes and send the reviewed draft to them",
	RunE:  runDraftMulticast,
}

func init() {
	multicastCmd.Flags().StringVar(&multicastPane, "pane", "", "Source pane id (default: current pane)")
	multicastCmd.Flags().BoolVar(&multicastAllPanes, "all-panes", false, "Show all panes, not just detected coding panes")
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

func continueMulticastStub(sourcePane string, targetIDs []string) error {
	_ = tmux.DisplayMessage(fmt.Sprintf("tmcmt: selected %d target%s for %s", len(targetIDs), plural(len(targetIDs)), sourcePane))
	return nil
}
