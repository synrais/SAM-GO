package attract

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/synrais/SAM-GO/pkg/games"
)

//
// System/group helpers
//

// GetSystemsByCategory retrieves systems by category (Console, Handheld, Arcade, etc.).
func GetSystemsByCategory(category string) ([]string, error) {
	var systemIDs []string
	for _, systemID := range games.AllSystems() {
		if strings.EqualFold(systemID.Category, category) {
			systemIDs = append(systemIDs, systemID.Id)
		}
	}
	if len(systemIDs) == 0 {
		return nil, fmt.Errorf("no systems found in category: %s", category)
	}
	return systemIDs, nil
}

// ExpandGroups expands category/group names into system IDs.
func ExpandGroups(systemIDs []string) ([]string, error) {
	var expanded []string
	for _, systemID := range systemIDs {
		trimmed := strings.TrimSpace(systemID)
		if trimmed == "" {
			continue
		}

		if trimmed == "Console" || trimmed == "Handheld" || trimmed == "Arcade" || trimmed == "Computer" {
			groupSystems, err := GetSystemsByCategory(trimmed)
			if err != nil {
				return nil, fmt.Errorf("group not found: %v", trimmed)
			}
			expanded = append(expanded, groupSystems...)
			continue
		}

		if sys, err := games.LookupSystem(trimmed); err == nil {
			expanded = append(expanded, sys.Id)
			continue
		}

		expanded = append(expanded, trimmed)
	}
	return expanded, nil
}

// FilterAllowed applies include/exclude restrictions case-insensitively.
func FilterAllowed(all []string, include, exclude []string) []string {
	var filtered []string
	for _, sys := range all {
		base := strings.TrimSuffix(filepath.Base(sys), "_gamelist.txt")
		if len(include) > 0 {
			if !ContainsInsensitive(include, base) {
				continue
			}
		}
		if ContainsInsensitive(exclude, base) {
			continue
		}
		filtered = append(filtered, sys)
	}
	return filtered
}
