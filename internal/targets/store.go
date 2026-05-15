package targets

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type store map[string][]string

func path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "tmcmt", "targets.json"), nil
}

// Save remembers the destination panes last selected for a source pane.
// Passing no targets clears that source entry while preserving other sources.
func Save(sourcePane string, targetPanes []string) error {
	path, err := path()
	if err != nil {
		return err
	}

	state, err := read(path)
	if err != nil {
		return err
	}
	if len(targetPanes) == 0 {
		delete(state, sourcePane)
	} else {
		state[sourcePane] = unique(targetPanes)
	}
	return write(path, state)
}

// Load returns remembered destination panes for a source pane, optionally
// filtering the result to panes present in the provided live set.
func Load(sourcePane string, live map[string]struct{}) ([]string, error) {
	path, err := path()
	if err != nil {
		return nil, err
	}
	state, err := read(path)
	if err != nil {
		return nil, err
	}

	var out []string
	for _, paneID := range state[sourcePane] {
		if live != nil {
			if _, ok := live[paneID]; !ok {
				continue
			}
		}
		out = append(out, paneID)
	}
	return out, nil
}

func read(path string) (store, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return store{}, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return store{}, nil
	}

	var state store
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("read target state %s: %w", path, err)
	}
	if state == nil {
		state = store{}
	}
	return state, nil
}

func write(path string, state store) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func unique(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
