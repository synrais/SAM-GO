package attract

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/utils"
)

//
// Line helpers
//

// ParseLine extracts <timestamp> prefix and remainder.
func ParseLine(line string) (float64, string) {
	if strings.HasPrefix(line, "<") {
		if idx := strings.Index(line, ">"); idx > 1 {
			tsStr := line[1:idx]
			rest := line[idx+1:]
			ts, _ := strconv.ParseFloat(tsStr, 64)
			return ts, rest
		}
	}
	return 0, line
}

// ReadStaticTimestamp returns the static timestamp for a game if present.
func ReadStaticTimestamp(systemID, game string) float64 {
	filterBase := config.FilterlistDir()
	path := filepath.Join(filterBase, systemID+"_staticlist.txt")

	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		tsStr := strings.Trim(parts[0], "<>")
		name, _ := utils.NormalizeEntry(parts[1])
		if name == game {
			ts, _ := strconv.ParseFloat(tsStr, 64)
			return ts
		}
	}
	return 0
}
