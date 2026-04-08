package cmd

import "github.com/spf13/cobra"

var draftCmd = &cobra.Command{
	Use:   "draft",
	Short: "Manage per-pane comment drafts",
}

func init() {
	rootCmd.AddCommand(draftCmd)
}
