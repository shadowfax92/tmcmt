package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"tmcmt/internal/chunk"
	"tmcmt/internal/draft"
	"tmcmt/internal/sanitize"
	"tmcmt/internal/tmux"

	"github.com/spf13/cobra"
)

var addPane string

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Append a commented selection to the draft for --pane",
	Long: `Reads the selection from stdin (piped by tmux copy-pipe-no-clear),
opens an nvim popup to collect a comment, and appends the result to
the draft file for the target pane.

Does NOT paste into the pane. Use 'tmcmt draft flush' to send.`,
	RunE: runDraftAdd,
}

func init() {
	addCmd.Flags().StringVar(&addPane, "pane", "", "Target pane id (default: current pane)")
	draftCmd.AddCommand(addCmd)
}

func runDraftAdd(cmd *cobra.Command, args []string) error {
	if !tmux.IsInsideTmux() {
		return errors.New("not inside a tmux session (TMUX env not set)")
	}

	paneID, err := resolvePane(addPane)
	if err != nil {
		return err
	}

	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}
	selection := sanitize.Clean(string(raw))
	if selection == "" {
		return errors.New("empty selection on stdin")
	}

	tmpl := chunk.Build(selection)
	chunkPath, err := chunk.WriteTempFile(tmpl)
	if err != nil {
		return err
	}
	defer os.Remove(chunkPath)

	if err := tmux.EditInPopup(chunkPath, true); err != nil {
		return fmt.Errorf("popup nvim: %w", err)
	}

	comment, changed, err := chunk.ParseFile(chunkPath, tmpl)
	if err != nil {
		return fmt.Errorf("parse chunk: %w", err)
	}
	if !changed {
		_ = tmux.DisplayMessage("tmcmt: chunk cancelled — no changes")
		return nil
	}

	if err := draft.Append(paneID, strings.TrimSpace(comment), selection); err != nil {
		return fmt.Errorf("append draft: %w", err)
	}

	_ = tmux.ClearSelection(paneID)
	_ = draft.GC()

	count, _ := draft.ChunkCount(paneID)
	msg := fmt.Sprintf("tmcmt: %d chunk%s in draft — C to flush", count, plural(count))
	_ = tmux.DisplayMessage(msg)

	return nil
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func resolvePane(flag string) (string, error) {
	if flag != "" {
		return flag, nil
	}
	id, err := tmux.CurrentPaneID()
	if err != nil {
		return "", fmt.Errorf("get current pane: %w", err)
	}
	return id, nil
}
