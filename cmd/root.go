package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var Version = "dev"

var ErrCancelled = errors.New("")

var rootCmd = &cobra.Command{
	Use:   "tmcmt",
	Short: "Compose commented chunks of tmux pane output for AI agents",
	Long: `tmcmt accumulates commented chunks of tmux pane output into a per-pane
draft, then pastes the whole draft into the pane's prompt with one keystroke.

Built for long-form replies to running claude / codex agents from inside
their own tmux pane, without losing copy-mode scroll position.

Typical flow inside an agent pane:

  1. M-w enter copy mode, scroll up
  2. v select a passage, c append a commented chunk (nvim popup)
  3. repeat 2 as many times as you like — scroll position is preserved
  4. C flush the draft: review in nvim popup, :wq, paste into pane
  5. hit Enter yourself when the prompt looks right`,
	Version:       Version,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		if errors.Is(err, ErrCancelled) {
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, "tmcmt:", err)
		os.Exit(1)
	}
}
