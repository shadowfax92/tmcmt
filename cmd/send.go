package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"

	"tmcmt/internal/tmux"

	"github.com/spf13/cobra"
)

var (
	sendPane  string
	sendEnter bool
)

var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Paste stdin into --pane (scriptable primitive)",
	Long: `Reads stdin and pastes it into the target pane's prompt via bracketed
paste. Does not involve the draft system — useful for scripts, multicast
wrappers, or one-off automation.`,
	RunE: runSend,
}

func init() {
	sendCmd.Flags().StringVar(&sendPane, "pane", "", "Target pane id (default: current pane)")
	sendCmd.Flags().BoolVar(&sendEnter, "enter", false, "Press Enter after pasting")
	rootCmd.AddCommand(sendCmd)
}

func runSend(cmd *cobra.Command, args []string) error {
	if !tmux.IsInsideTmux() {
		return errors.New("not inside a tmux session")
	}
	paneID, err := resolvePane(sendPane)
	if err != nil {
		return err
	}
	content, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}
	if len(content) == 0 {
		return errors.New("empty stdin")
	}
	if err := tmux.PasteToPane(paneID, string(content)); err != nil {
		return err
	}
	if sendEnter {
		return tmux.SendEnter(paneID)
	}
	return nil
}
