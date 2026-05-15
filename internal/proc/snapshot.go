package proc

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Process struct {
	PID  int
	PPID int
	Args string
}

type Snapshot struct {
	procs    map[int]Process
	children map[int][]int
}

// NewSnapshot captures the current user's process table once so callers can
// inspect many pane process trees without shelling out per pane.
func NewSnapshot() (*Snapshot, error) {
	args := []string{"-ww", "-o", "pid,ppid,args"}
	if user := os.Getenv("USER"); user != "" {
		args = append([]string{"-u", user}, args...)
	}
	out, err := exec.Command("ps", args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ps %s: %s (%w)", strings.Join(args, " "), strings.TrimSpace(string(out)), err)
	}
	return ParseSnapshot(string(out))
}

// ParseSnapshot builds a process graph from ps output with pid, ppid, and args
// columns. It is exposed so pane discovery can be tested without live processes.
func ParseSnapshot(output string) (*Snapshot, error) {
	s := &Snapshot{
		procs:    map[int]Process{},
		children: map[int][]int{},
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.EqualFold(fields[0], "PID") && strings.EqualFold(fields[1], "PPID") {
			continue
		}
		if len(fields) < 3 {
			return nil, fmt.Errorf("malformed process row %q", line)
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			return nil, fmt.Errorf("parse pid in %q: %w", line, err)
		}
		ppid, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, fmt.Errorf("parse ppid in %q: %w", line, err)
		}
		s.procs[pid] = Process{
			PID:  pid,
			PPID: ppid,
			Args: strings.Join(fields[2:], " "),
		}
		s.children[ppid] = append(s.children[ppid], pid)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	for ppid := range s.children {
		sort.Ints(s.children[ppid])
	}
	return s, nil
}

// WalkDescendants visits pid and all descendants breadth-first. Returning true
// from the callback stops traversal after the current process.
func (s *Snapshot) WalkDescendants(pid int, visit func(Process) bool) {
	if s == nil || visit == nil {
		return
	}
	queue := []int{pid}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		p, ok := s.procs[current]
		if !ok {
			continue
		}
		if visit(p) {
			return
		}
		queue = append(queue, s.children[current]...)
	}
}

// ExecutableName returns argv[0]'s basename from a process args string.
func ExecutableName(args string) string {
	fields := strings.Fields(args)
	if len(fields) == 0 {
		return ""
	}
	return filepath.Base(fields[0])
}
