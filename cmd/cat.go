package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"tmcmt/internal/draft"

	"github.com/spf13/cobra"
)

var catCount int

var catCmd = &cobra.Command{
	Use:     "cat",
	Aliases: []string{"ls"},
	Short:   "Print recent flushed draft sessions",
	Long: `Print archived sessions from drafts/done/.

By default, prints the most recent flushed session. Use -n to print more
recent sessions, newest first.`,
	Args: cobra.NoArgs,
	RunE: runCat,
}

func init() {
	catCmd.Flags().IntVarP(&catCount, "count", "n", 1, "Number of done sessions to print")
	rootCmd.AddCommand(catCmd)
}

// runCat prints archived draft sessions so flushed prompts can be inspected
// without knowing the on-disk archive path.
func runCat(cmd *cobra.Command, args []string) error {
	if catCount <= 0 {
		return errors.New("-n must be positive")
	}

	archives, err := draft.ListArchives(catCount)
	if err != nil {
		return err
	}

	w := cmd.OutOrStdout()
	if len(archives) == 0 {
		fmt.Fprintln(w, "no done sessions")
		return nil
	}

	for i, archive := range archives {
		fmt.Fprintf(w, "==> %s <==\n", archive.Path)

		content, err := os.ReadFile(archive.Path)
		if err != nil {
			return fmt.Errorf("read archive %s: %w", archive.Path, err)
		}
		text := string(content)
		fmt.Fprint(w, text)
		if i < len(archives)-1 {
			if !strings.HasSuffix(text, "\n") {
				fmt.Fprintln(w)
			}
			fmt.Fprintln(w)
		}
	}
	return nil
}
