package chunk

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The parser matches on this tag, not the exact separator line, so the
// surrounding decoration is purely cosmetic and can change freely.
const separatorTag = "TMCMT-SELECTION-BELOW"

const separatorLine = "# ─────────────── " + separatorTag + " ───────────────"

// Template is a chunk composition template written to disk for nvim to edit.
type Template struct {
	Selection string // sanitized selection, preserved verbatim
	Content   string // full template text written to the temp file
}

// Build constructs a chunk template with an empty editable area at the top
// and a commented-out preview of the selection below the separator.
func Build(selection string) Template {
	var b strings.Builder
	// Three blank lines give the user room to type a multi-line comment
	// without immediately running into the separator.
	b.WriteString("\n\n\n")
	b.WriteString(separatorLine)
	b.WriteString("\n")
	b.WriteString("# Selection preview (read-only; will be sent as a fenced block):\n")
	b.WriteString("#\n")
	for _, line := range strings.Split(strings.TrimRight(selection, "\n"), "\n") {
		b.WriteString("# ")
		b.WriteString(line)
		b.WriteString("\n")
	}
	b.WriteString("#\n")
	b.WriteString("# " + strings.Repeat("─", 75) + "\n")
	return Template{
		Selection: selection,
		Content:   b.String(),
	}
}

// WriteTempFile writes the template to a temp file and returns its path.
// Callers should os.Remove the returned path when done.
func WriteTempFile(t Template) (string, error) {
	f, err := os.CreateTemp(tempDir(), "tmcmt-chunk-*.md")
	if err != nil {
		return "", fmt.Errorf("create chunk temp file: %w", err)
	}
	defer f.Close()
	if _, err := f.WriteString(t.Content); err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// ParseFile reads the chunk file and extracts the comment (everything above
// the separator line). Returns changed=false if the file is byte-identical
// to the original template — i.e. the user exited the editor without
// touching anything, and the chunk should be treated as cancelled.
func ParseFile(path string, original Template) (comment string, changed bool, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false, err
	}
	if string(data) == original.Content {
		return "", false, nil
	}

	var commentLines []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, separatorTag) {
			break
		}
		commentLines = append(commentLines, line)
	}
	return strings.TrimSpace(strings.Join(commentLines, "\n")), true, nil
}

func tempDir() string {
	if d := os.Getenv("XDG_RUNTIME_DIR"); d != "" {
		td := filepath.Join(d, "tmcmt")
		if err := os.MkdirAll(td, 0o755); err == nil {
			return td
		}
	}
	td := filepath.Join(os.TempDir(), "tmcmt")
	if err := os.MkdirAll(td, 0o755); err == nil {
		return td
	}
	return os.TempDir()
}
