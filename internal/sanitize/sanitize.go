package sanitize

import (
	"regexp"
	"strings"
)

// Matches ANSI CSI sequences (colors, cursor movement, etc.).
var ansiCSI = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]`)

// Matches ANSI OSC sequences (title setting, hyperlinks, etc.), terminated
// by BEL or ST (ESC \).
var ansiOSC = regexp.MustCompile(`\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)`)

// Clean strips ANSI escape sequences and trims trailing whitespace per line.
// Leading and trailing empty lines are removed.
func Clean(s string) string {
	s = ansiCSI.ReplaceAllString(s, "")
	s = ansiOSC.ReplaceAllString(s, "")

	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t\r")
	}

	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	end := len(lines)
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	if start >= end {
		return ""
	}
	return strings.Join(lines[start:end], "\n")
}
