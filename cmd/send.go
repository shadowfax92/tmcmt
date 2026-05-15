package cmd

import (
	"errors"
	"fmt"
	"io"

	"tmcmt/internal/tmux"

	"github.com/spf13/cobra"
)

var (
	sendPanes []string
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
	sendCmd.Flags().StringSliceVar(&sendPanes, "pane", nil, "Target pane id (default: current pane, repeatable or comma-separated)")
	sendCmd.Flags().BoolVar(&sendEnter, "enter", false, "Press Enter after pasting")
	rootCmd.AddCommand(sendCmd)
}

func runSend(cmd *cobra.Command, args []string) error {
	if !tmux.IsInsideTmux() {
		return errors.New("not inside a tmux session")
	}
	targetIDs, err := resolveSendTargets()
	if err != nil {
		return err
	}
	content, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}
	if len(content) == 0 {
		return errors.New("empty stdin")
	}
	for _, paneID := range targetIDs {
		if err := pasteToPane(paneID, string(content)); err != nil {
			return err
		}
	}
	if sendEnter {
		for _, paneID := range targetIDs {
			if err := sendEnterToPane(paneID); err != nil {
				return err
			}
		}
	}
	return nil
}

func resolveSendTargets() ([]string, error) {
	if len(sendPanes) == 0 {
		paneID, err := resolvePane("")
		if err != nil {
			return nil, err
		}
		return []string{paneID}, nil
	}
	return parsePaneList(sendPanes)
}
