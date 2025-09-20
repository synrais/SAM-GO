package attract

import (
	"math/rand"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/synrais/SAM-GO/pkg/config"
)

//
// Extra helpers
//

// ParsePlayTime handles "40" or "40-130" style configs.
func ParsePlayTime(value string, r *rand.Rand) time.Duration {
	if strings.Contains(value, "-") {
		parts := strings.SplitN(value, "-", 2)
		min, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
		max, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
		if max > min {
			return time.Duration(r.Intn(max-min+1)+min) * time.Second
		}
		return time.Duration(min) * time.Second
	}
	secs, _ := strconv.Atoi(value)
	return time.Duration(secs) * time.Second
}

// Disabled checks if a game should be blocked by disable rules.
func Disabled(systemID string, gamePath string, cfg *config.UserConfig) bool {
	rules, ok := cfg.Disable[systemID]
	if !ok {
		return false
	}

	base := filepath.Base(gamePath)
	ext := filepath.Ext(gamePath)
	dir := filepath.Base(filepath.Dir(gamePath))

	for _, f := range rules.Folders {
		if matchRule(f, dir) {
			return true
		}
	}
	for _, f := range rules.Files {
		if matchRule(f, base) {
			return true
		}
	}
	for _, e := range rules.Extensions {
		if strings.EqualFold(ext, e) {
			return true
		}
	}
	return false
}
