package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func run(args ...string) (string, error) {
	cmd := exec.Command("tmux", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("tmux %s: %s (%w)",
			strings.Join(args, " "),
			strings.TrimSpace(string(out)),
			err)
	}
	return strings.TrimSpace(string(out)), nil
}

func IsInsideTmux() bool {
	return os.Getenv("TMUX") != ""
}

func CurrentPaneID() (string, error) {
	return run("display-message", "-p", "#{pane_id}")
}

func PaneExists(paneID string) bool {
	_, err := run("display-message", "-t", paneID, "-p", "")
	return err == nil
}

// AlivePaneIDs returns the set of currently-alive pane ids ("%42") across
// the entire tmux server.
func AlivePaneIDs() (map[string]struct{}, error) {
	out, err := run("list-panes", "-a", "-F", "#{pane_id}")
	if err != nil {
		return nil, err
	}
	set := make(map[string]struct{})
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			set[line] = struct{}{}
		}
	}
	return set, nil
}

// ClearSelection clears the copy-mode selection in the target pane without
// exiting copy mode.
func ClearSelection(paneID string) error {
	_, err := run("send-keys", "-t", paneID, "-X", "clear-selection")
	return err
}

// DisplayMessage shows a transient message in tmux's status line.
func DisplayMessage(msg string) error {
	_, err := run("display-message", msg)
	return err
}

// SendEnter presses Enter in the target pane.
func SendEnter(paneID string) error {
	_, err := run("send-keys", "-t", paneID, "Enter")
	return err
}

// EditInPopup launches $EDITOR (or nvim) on the given file inside a tmux
// display-popup and blocks until the editor exits. If insertMode is true
// and the editor is nvim/vim, the editor starts in insert mode at line 1.
func EditInPopup(path string, insertMode bool) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "nvim"
	}

	var shellCmd string
	if insertMode && isVimLike(editor) {
		shellCmd = fmt.Sprintf(`TMCMT=1 %s -c 'normal! gg' -c 'startinsert' %s`,
			editor, shellQuote(path))
	} else {
		shellCmd = fmt.Sprintf(`TMCMT=1 %s %s`, editor, shellQuote(path))
	}

	_, err := run("display-popup", "-E", "-w", "80%", "-h", "80%", shellCmd)
	return err
}

func isVimLike(editor string) bool {
	e := strings.ToLower(editor)
	return strings.Contains(e, "nvim") || strings.Contains(e, "vim")
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// PasteToPane loads content into a tmux buffer and pastes it into the target
// pane using bracketed paste (-p). The buffer is deleted after the paste (-d)
// so it does not leak into the paste-buffer list.
func PasteToPane(paneID, content string) error {
	cmd := exec.Command("tmux", "load-buffer", "-b", "tmcmt", "-")
	cmd.Stdin = strings.NewReader(content)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("load-buffer: %s (%w)",
			strings.TrimSpace(string(out)), err)
	}
	if _, err := run("paste-buffer", "-b", "tmcmt", "-p", "-d", "-t", paneID); err != nil {
		return err
	}
	return nil
}
