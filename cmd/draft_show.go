package cmd

import (
	"fmt"
	"os"

	"tmcmt/internal/draft"

	"github.com/spf13/cobra"
)

var showPane string

var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Print the current draft for --pane to stdout",
	RunE:  runDraftShow,
}

func init() {
	showCmd.Flags().StringVar(&showPane, "pane", "", "Target pane id (default: current pane)")
	draftCmd.AddCommand(showCmd)
}

func runDraftShow(cmd *cobra.Command, args []string) error {
	paneID, err := resolvePane(showPane)
	if err != nil {
		return err
	}
	path, exists := draft.PathIfExists(paneID)
	if !exists {
		fmt.Fprintf(os.Stderr, "no draft for %s\n", paneID)
		return nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	fmt.Print(string(content))
	return nil
}
