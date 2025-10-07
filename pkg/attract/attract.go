package attract

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/games"
	"github.com/synrais/SAM-GO/pkg/gamesdb"
	"github.com/synrais/SAM-GO/pkg/mister"
)

// StartAttractMode runs an endless random game loop using
// the already-loaded menu database. It reads Attract mode
// settings from SAM.ini (using embedded default.ini if missing).
func StartAttractMode(files []gamesdb.FileInfo) error {
	fmt.Println("=== Starting Attract Mode ===")

	// Load our embedded SAM.ini configuration
	cfg, err := config.LoadINI()
	if err != nil {
		return fmt.Errorf("failed to load SAM.ini: %w", err)
	}

	filtered := filterSystems(files, cfg)
	if len(filtered) == 0 {
		return fmt.Errorf("no games available after filtering")
	}

	// --- Inline playtime parsing ---
	minTime, maxTime := 40, 40
	raw := strings.TrimSpace(cfg.Attract.PlayTime)
	if raw != "" {
		if strings.Contains(raw, "-") {
			var a, b int
			fmt.Sscanf(raw, "%d-%d", &a, &b)
			if a > 0 {
				minTime = a
			}
			if b >= a {
				maxTime = b
			} else {
				maxTime = minTime
			}
		} else {
			var v int
			fmt.Sscanf(raw, "%d", &v)
			if v > 0 {
				minTime, maxTime = v, v
			}
		}
	}
	// ------------------------------

	rand.Seed(time.Now().UnixNano())

	for {
		// Shuffle each cycle if Random = true
		if cfg.Attract.Random {
			rand.Shuffle(len(filtered), func(i, j int) {
				filtered[i], filtered[j] = filtered[j], filtered[i]
			})
		}

		for _, g := range filtered {
			sys, err := games.GetSystem(g.SystemId)
			if err != nil {
				continue
			}

			display := g.Name
			if g.Ext != "" {
				display += "." + g.Ext
			}

			fmt.Printf("[Attract] Launching %s (%s)\n", display, sys.Name)
			_ = mister.LaunchGame(nil, *sys, g.Path)

			// Inline random playtime
			playTime := minTime
			if minTime != maxTime {
				playTime = rand.Intn(maxTime-minTime+1) + minTime
			}
			time.Sleep(time.Duration(playTime) * time.Second)
		}
	}
}

// filterSystems filters the games slice using Include/Exclude
// rules from the Attract section of SAM.ini
func filterSystems(files []gamesdb.FileInfo, cfg *config.Config) []gamesdb.FileInfo {
	var out []gamesdb.FileInfo
	include := make(map[string]bool)
	exclude := make(map[string]bool)

	for _, s := range cfg.Attract.Include {
		include[strings.ToLower(strings.TrimSpace(s))] = true
	}
	for _, s := range cfg.Attract.Exclude {
		exclude[strings.ToLower(strings.TrimSpace(s))] = true
	}

	for _, f := range files {
		sys := strings.ToLower(f.SystemId)
		if len(include) > 0 && !include[sys] {
			continue
		}
		if exclude[sys] {
			continue
		}
		out = append(out, f)
	}
	return out
}
