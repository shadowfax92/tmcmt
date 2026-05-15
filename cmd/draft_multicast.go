package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"tmcmt/internal/draft"
	"tmcmt/internal/targets"
	"tmcmt/internal/tmux"

	"github.com/spf13/cobra"
)

var (
	multicastPane     string
	multicastAllPanes bool
	multicastSend     bool
	multicastTargets  []string
	multicastReuse    bool
	multicastDryRun   bool

	discoverMulticastCandidates = tmux.DiscoverCodingPanes
	selectMulticastTargets      = tmux.SelectCandidates
	continueMulticast           = runMulticastDispatch
	alivePaneIDs                = tmux.AlivePaneIDs
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
	multicastCmd.Flags().StringSliceVar(&multicastTargets, "targets", nil, "Comma-separated destination pane ids")
	multicastCmd.Flags().BoolVar(&multicastReuse, "reuse", false, "Reuse remembered live targets without opening the selector")
	multicastCmd.Flags().BoolVar(&multicastDryRun, "dry-run", false, "Print targets and payload instead of pasting")
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

	targetIDs, remember, err := resolveMulticastTargets(sourcePane)
	if err != nil {
		return err
	}
	if len(targetIDs) == 0 {
		return ErrCancelled
	}
	if multicastDryRun {
		return printMulticastDryRun(cmd, sourcePane, targetIDs)
	}
	if remember {
		if err := targets.Save(sourcePane, targetIDs); err != nil {
			return err
		}
	}
	return continueMulticast(sourcePane, targetIDs)
}

func resolveMulticastTargets(sourcePane string) ([]string, bool, error) {
	if len(multicastTargets) > 0 {
		targetIDs, err := parsePaneList(multicastTargets)
		return targetIDs, true, err
	}
	if multicastReuse {
		live, err := alivePaneIDs()
		if err != nil {
			return nil, false, err
		}
		targetIDs, err := targets.Load(sourcePane, live)
		if err != nil {
			return nil, false, err
		}
		if len(targetIDs) == 0 {
			return nil, false, errors.New("no remembered live targets")
		}
		return targetIDs, false, nil
	}

	candidates, err := discoverMulticastCandidates(sourcePane, multicastAllPanes)
	if err != nil {
		return nil, false, err
	}
	if len(candidates) == 0 {
		return nil, false, errors.New("no target panes found")
	}

	live := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		live[candidate.Pane.ID] = struct{}{}
	}
	remembered, err := targets.Load(sourcePane, live)
	if err != nil {
		return nil, false, err
	}
	targetIDs, err := selectMulticastTargets(candidates, remembered)
	if errors.Is(err, tmux.ErrSelectionCancelled) {
		return nil, false, ErrCancelled
	}
	return targetIDs, true, err
}

func printMulticastDryRun(cmd *cobra.Command, sourcePane string, targetIDs []string) error {
	path, exists := draft.PathIfExists(sourcePane)
	if !exists {
		return fmt.Errorf("no draft for %s", sourcePane)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read draft: %w", err)
	}
	if strings.TrimSpace(string(content)) == "" {
		return errors.New("draft is empty")
	}
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Targets: %s\n\n%s", strings.Join(targetIDs, ", "), string(content))
	return nil
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
