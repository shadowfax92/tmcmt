package draft

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"tmcmt/internal/tmux"
)

const archiveRetentionLimit = 1000

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

func doneDir() (string, error) {
	draftsDir, err := dir()
	if err != nil {
		return "", err
	}
	d := filepath.Join(draftsDir, "done")
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

// Archive moves the active pane draft into done/ and returns its new path.
func Archive(paneID string) (string, error) {
	src, err := pathFor(paneID)
	if err != nil {
		return "", err
	}
	dst, err := nextArchivePath(paneID)
	if err != nil {
		return "", err
	}
	if err := os.Rename(src, dst); err != nil {
		return "", err
	}
	now := time.Now()
	if err := os.Chtimes(dst, now, now); err != nil {
		return "", err
	}
	if err := pruneArchives(); err != nil {
		return "", err
	}
	return dst, nil
}

func nextArchivePath(paneID string) (string, error) {
	d, err := doneDir()
	if err != nil {
		return "", err
	}
	prefix := sanitizePaneID(paneID) + "-"
	entries, err := os.ReadDir(d)
	if err != nil {
		return "", err
	}

	maxSeq := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), prefix) || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		seq, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(e.Name(), prefix), ".md"))
		if err == nil && seq > maxSeq {
			maxSeq = seq
		}
	}
	return filepath.Join(d, fmt.Sprintf("%s%06d.md", prefix, maxSeq+1)), nil
}

func pruneArchives() error {
	d, err := doneDir()
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(d)
	if err != nil {
		return err
	}

	type archiveFile struct {
		name    string
		path    string
		modTime int64
	}
	var files []archiveFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, archiveFile{
			name:    e.Name(),
			path:    filepath.Join(d, e.Name()),
			modTime: info.ModTime().UnixNano(),
		})
	}
	if len(files) <= archiveRetentionLimit {
		return nil
	}

	sort.Slice(files, func(i, j int) bool {
		if files[i].modTime == files[j].modTime {
			return files[i].name < files[j].name
		}
		return files[i].modTime < files[j].modTime
	})

	for _, f := range files[:len(files)-archiveRetentionLimit] {
		if err := os.Remove(f.path); err != nil {
			return err
		}
	}
	return nil
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

// ArchiveInfo describes a flushed draft stored under drafts/done/.
type ArchiveInfo struct {
	Path    string
	Size    int64
	ModTime time.Time
}

// ListArchives returns the newest archived drafts from drafts/done/.
// Archives are ordered by mtime, with filename as a deterministic tie-breaker.
func ListArchives(limit int) ([]ArchiveInfo, error) {
	if limit <= 0 {
		return nil, errors.New("archive limit must be positive")
	}

	d, err := doneDir()
	if err != nil {
		return nil, err
	}
	d, err = filepath.Abs(d)
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

	type archiveFile struct {
		name string
		info ArchiveInfo
	}
	var files []archiveFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		stat, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, archiveFile{
			name: e.Name(),
			info: ArchiveInfo{
				Path:    filepath.Join(d, e.Name()),
				Size:    stat.Size(),
				ModTime: stat.ModTime(),
			},
		})
	}

	sort.Slice(files, func(i, j int) bool {
		if files[i].info.ModTime.Equal(files[j].info.ModTime) {
			return files[i].name > files[j].name
		}
		return files[i].info.ModTime.After(files[j].info.ModTime)
	})

	if len(files) > limit {
		files = files[:limit]
	}

	out := make([]ArchiveInfo, 0, len(files))
	for _, file := range files {
		out = append(out, file.info)
	}
	return out, nil
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
