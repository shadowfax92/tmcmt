package draft

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"tmcmt/internal/tmux"
)

func dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	d := filepath.Join(home, ".local", "state", "tmcmt", "drafts")
	if err := os.MkdirAll(d, 0o755); err != nil {
		return "", err
	}
	return d, nil
}

// Pane ids look like "%42"; strip the % so the filename is clean and
// filesystem-safe.
func sanitizePaneID(paneID string) string {
	return strings.TrimPrefix(paneID, "%")
}

func pathFor(paneID string) (string, error) {
	d, err := dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, sanitizePaneID(paneID)+".md"), nil
}

// PathIfExists returns the draft path and whether it is present and non-empty.
func PathIfExists(paneID string) (string, bool) {
	p, err := pathFor(paneID)
	if err != nil {
		return "", false
	}
	info, err := os.Stat(p)
	if err != nil {
		return p, false
	}
	return p, info.Size() > 0
}

// Append adds a new chunk (comment + fenced selection) to the pane's draft.
// An empty comment is allowed — the chunk will consist of the fenced
// selection only.
func Append(paneID, comment, selection string) error {
	p, err := pathFor(paneID)
	if err != nil {
		return err
	}

	var b strings.Builder
	if comment != "" {
		b.WriteString(strings.TrimRight(comment, "\n"))
		b.WriteString("\n\n")
	}
	b.WriteString("```\n")
	b.WriteString(strings.TrimRight(selection, "\n"))
	b.WriteString("\n```\n\n")

	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(b.String())
	return err
}

// ChunkCount approximates the number of chunks in a draft by counting code
// fence pairs. Good enough for a status message; not authoritative.
func ChunkCount(paneID string) (int, error) {
	p, ok := PathIfExists(paneID)
	if !ok {
		return 0, nil
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return 0, err
	}
	fences := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "```" {
			fences++
		}
	}
	return fences / 2, nil
}

type Info struct {
	PaneID     string
	Path       string
	Size       int64
	ChunkCount int
}

func List() ([]Info, error) {
	d, err := dir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(d)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var out []Info
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		paneID := "%" + strings.TrimSuffix(e.Name(), ".md")
		cc, _ := ChunkCount(paneID)
		out = append(out, Info{
			PaneID:     paneID,
			Path:       filepath.Join(d, e.Name()),
			Size:       info.Size(),
			ChunkCount: cc,
		})
	}
	return out, nil
}

// GC deletes draft files whose pane id no longer exists in the tmux server.
// Failures are swallowed — GC is opportunistic, not load-bearing.
func GC() error {
	alive, err := tmux.AlivePaneIDs()
	if err != nil {
		return err
	}
	drafts, err := List()
	if err != nil {
		return err
	}
	for _, d := range drafts {
		if _, ok := alive[d.PaneID]; !ok {
			_ = os.Remove(d.Path)
		}
	}
	return nil
}
