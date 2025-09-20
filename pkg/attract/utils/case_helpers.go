package attract

import "strings"

//
// Case-insensitive helpers
//

// ContainsInsensitive checks if list contains item, ignoring case/whitespace.
func ContainsInsensitive(list []string, item string) bool {
	for _, v := range list {
		if strings.EqualFold(strings.TrimSpace(v), item) {
			return true
		}
	}
	return false
}

// MatchesSystem is a wrapper for ContainsInsensitive, for system IDs.
func MatchesSystem(list []string, system string) bool {
	return ContainsInsensitive(list, system)
}

// AllowedFor checks include/exclude rules for a system ID.
func AllowedFor(system string, include, exclude []string) bool {
	if len(include) > 0 && !MatchesSystem(include, system) {
		return false
	}
	if MatchesSystem(exclude, system) {
		return false
	}
	return true
}
