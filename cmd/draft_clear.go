package cmd

import (
	"fmt"
	"os"

	"tmcmt/internal/draft"
	"tmcmt/internal/tmux"

	"github.com/spf13/cobra"
)

var clearPane string

var clearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Discard the current draft for --pane",
	RunE:  runDraftClear,
}

func init() {
	clearCmd.Flags().StringVar(&clearPane, "pane", "", "Target pane id (default: current pane)")
	draftCmd.AddCommand(clearCmd)
}

func runDraftClear(cmd *cobra.Command, args []string) error {
	paneID, err := resolvePane(clearPane)
	if err != nil {
		return err
	}
	path, exists := draft.PathIfExists(paneID)
	if !exists {
		fmt.Fprintf(os.Stderr, "no draft for %s\n", paneID)
		return nil
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	_ = tmux.DisplayMessage(fmt.Sprintf("tmcmt: cleared draft for %s", paneID))
	return nil
}
