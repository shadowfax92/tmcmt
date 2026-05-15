package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"tmcmt/internal/draft"
	"tmcmt/internal/tmux"

	"github.com/spf13/cobra"
)

var (
	flushPane     string
	flushNoReview bool
	flushSend     bool
	flushDryRun   bool

	editDraftInPopup = tmux.EditInPopup
	pasteToPane      = tmux.PasteToPane
	sendEnterToPane  = tmux.SendEnter
	paneStillExists  = tmux.PaneExists
	archiveDraft     = draft.Archive
)

var flushCmd = &cobra.Command{
	Use:   "flush",
	Short: "Paste the accumulated draft into --pane and archive it",
	Long: `Opens the draft in an nvim popup for a final review/edit pass,
then pastes the whole draft into the target pane via bracketed paste
and moves the draft file into drafts/done/.

Does not press Enter unless --send. The pasted content sits in the
agent's prompt for you to review and submit manually.`,
	RunE: runDraftFlush,
}

func init() {
	flushCmd.Flags().StringVar(&flushPane, "pane", "", "Target pane id (default: current pane)")
	flushCmd.Flags().BoolVar(&flushNoReview, "no-review", false, "Skip nvim review; paste draft as-is")
	flushCmd.Flags().BoolVar(&flushSend, "send", false, "Press Enter after pasting")
	flushCmd.Flags().BoolVar(&flushDryRun, "dry-run", false, "Print final payload to stdout instead of pasting")
	draftCmd.AddCommand(flushCmd)
}

func runDraftFlush(cmd *cobra.Command, args []string) error {
	if !tmux.IsInsideTmux() && !flushDryRun {
		return errors.New("not inside a tmux session (TMUX env not set)")
	}

	paneID, err := resolvePane(flushPane)
	if err != nil {
		return err
	}

	path, exists := draft.PathIfExists(paneID)
	if !exists {
		_ = tmux.DisplayMessage(fmt.Sprintf("tmcmt: no draft for %s", paneID))
		return nil
	}

	content, ok, err := reviewDraftFile(path, !flushNoReview && !flushDryRun)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	if flushDryRun {
		fmt.Print(content)
		return nil
	}

	if err := pasteToPane(paneID, content); err != nil {
		return fmt.Errorf("paste: %w", err)
	}

	if flushSend {
		if err := sendEnterToPane(paneID); err != nil {
			return fmt.Errorf("send enter: %w", err)
		}
	}

	if _, err := archiveDraft(paneID); err != nil {
		return fmt.Errorf("archive draft: %w", err)
	}

	_ = tmux.DisplayMessage("tmcmt: draft flushed and archived")
	return nil
}

func reviewDraftFile(path string, review bool) (string, bool, error) {
	if review {
		if err := editDraftInPopup(path, false); err != nil {
			return "", false, fmt.Errorf("popup nvim: %w", err)
		}
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", false, fmt.Errorf("read draft: %w", err)
	}
	if strings.TrimSpace(string(content)) == "" {
		_ = os.Remove(path)
		_ = tmux.DisplayMessage("tmcmt: draft empty, cleared")
		return "", false, nil
	}
	return string(content), true, nil
}
