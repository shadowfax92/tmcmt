package cmd

import (
	"fmt"

	"tmcmt/internal/draft"
	"tmcmt/internal/tmux"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all drafts with pane status",
	RunE:  runDraftList,
}

func init() {
	draftCmd.AddCommand(listCmd)
}

func runDraftList(cmd *cobra.Command, args []string) error {
	drafts, err := draft.List()
	if err != nil {
		return err
	}
	if len(drafts) == 0 {
		fmt.Println("no drafts")
		return nil
	}

	alive := map[string]struct{}{}
	if set, err := tmux.AlivePaneIDs(); err == nil {
		alive = set
	}

	fmt.Printf("%-10s  %-8s  %-12s  %s\n", "PANE", "CHUNKS", "SIZE", "STATUS")
	for _, d := range drafts {
		status := "alive"
		if _, ok := alive[d.PaneID]; !ok {
			status = "stale"
		}
		fmt.Printf("%-10s  %-8d  %-12d  %s\n", d.PaneID, d.ChunkCount, d.Size, status)
	}
	return nil
}
