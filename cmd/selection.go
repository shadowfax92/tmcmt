package cmd

import (
	"errors"
	"fmt"
	"io"

	"tmcmt/internal/targets"
	"tmcmt/internal/tmux"

	"github.com/spf13/cobra"
)

var selectionPane string

var selectionCmd = &cobra.Command{
	Use:   "selection",
	Short: "Send selected tmux text to remembered coding panes",
}

var selectionSendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send raw selected text to remembered coding panes",
	RunE:  runSelectionSend,
}

var selectionTargetsCmd = &cobra.Command{
	Use:   "targets",
	Short: "Select remembered coding panes for selection sends",
	RunE:  runSelectionTargets,
}

func init() {
	selectionSendCmd.Flags().StringVar(&selectionPane, "pane", "", "Source pane id (default: current pane)")
	selectionTargetsCmd.Flags().StringVar(&selectionPane, "pane", "", "Source pane id (default: current pane)")
	selectionCmd.AddCommand(selectionSendCmd, selectionTargetsCmd)
	rootCmd.AddCommand(selectionCmd)
}

// runSelectionSend sends raw stdin to remembered live targets for the source
// pane, only opening the selector when no remembered live target exists.
func runSelectionSend(cmd *cobra.Command, args []string) error {
	content, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}
	if len(content) == 0 {
		return errors.New("empty stdin")
	}

	sourcePane, err := resolvePane(selectionPane)
	if err != nil {
		return err
	}
	targetIDs, remember, err := resolveSelectionSendTargets(sourcePane)
	if err != nil {
		return err
	}
	if remember {
		if err := targets.Save(sourcePane, targetIDs); err != nil {
			return err
		}
	}
	if err := pasteSelectionToTargets(targetIDs, string(content)); err != nil {
		return err
	}
	_ = tmux.DisplayMessage(fmt.Sprintf("tmcmt: selection sent to %d target%s", len(targetIDs), plural(len(targetIDs))))
	return nil
}

// runSelectionTargets updates the remembered target panes without reading a
// selection or dispatching content.
func runSelectionTargets(cmd *cobra.Command, args []string) error {
	sourcePane, err := resolvePane(selectionPane)
	if err != nil {
		return err
	}
	targetIDs, err := selectSelectionTargets(sourcePane)
	if err != nil {
		return err
	}
	if err := targets.Save(sourcePane, targetIDs); err != nil {
		return err
	}
	_ = tmux.DisplayMessage(fmt.Sprintf("tmcmt: remembered %d selection target%s", len(targetIDs), plural(len(targetIDs))))
	return nil
}

func resolveSelectionSendTargets(sourcePane string) ([]string, bool, error) {
	live, err := alivePaneIDs()
	if err != nil {
		return nil, false, err
	}
	remembered, err := targets.Load(sourcePane, live)
	if err != nil {
		return nil, false, err
	}
	if len(remembered) > 0 {
		return remembered, false, nil
	}

	targetIDs, err := selectSelectionTargets(sourcePane)
	if err != nil {
		return nil, false, err
	}
	return targetIDs, true, nil
}

func selectSelectionTargets(sourcePane string) ([]string, error) {
	candidates, err := discoverMulticastCandidates(sourcePane, false)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, errors.New("no target panes found")
	}

	live := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		live[candidate.Pane.ID] = struct{}{}
	}
	remembered, err := targets.Load(sourcePane, live)
	if err != nil {
		return nil, err
	}
	targetIDs, err := selectMulticastTargets(candidates, remembered)
	if errors.Is(err, tmux.ErrSelectionCancelled) {
		return nil, ErrCancelled
	}
	if err != nil {
		return nil, err
	}
	if len(targetIDs) == 0 {
		return nil, ErrCancelled
	}
	return targetIDs, nil
}

func pasteSelectionToTargets(targetIDs []string, content string) error {
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
	return nil
}
