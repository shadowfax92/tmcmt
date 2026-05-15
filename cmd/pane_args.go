package cmd

import (
	"errors"
	"strings"
)

func parsePaneList(values []string) ([]string, error) {
	seen := map[string]struct{}{}
	var panes []string
	for _, value := range values {
		parts := strings.Split(value, ",")
		for _, part := range parts {
			paneID := strings.TrimSpace(part)
			if paneID == "" {
				return nil, errors.New("empty target pane")
			}
			if _, ok := seen[paneID]; ok {
				continue
			}
			seen[paneID] = struct{}{}
			panes = append(panes, paneID)
		}
	}
	return panes, nil
}
