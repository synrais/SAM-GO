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

// StartAttractMode runs an endless random game loop using config rules.
// It continues indefinitely until the process is stopped.
func StartAttractMode(userCfg *config.UserConfig) error {
	// Load Attract config (from SAM.ini or embedded default)
	cfg, err := config.LoadINI()
	if err != nil {
		return fmt.Errorf("failed to load attract config: %w", err)
	}

	fmt.Println("=== Starting Attract Mode ===")

	allGames, err := gamesdb.LoadAllGames()
	if err != nil {
		return fmt.Errorf("failed to load games database: %w", err)
	}

	filtered := filterSystems(allGames, cfg)
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
		// Shuffle for each cycle if Random enabled
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
			_ = mister.LaunchGame(userCfg, *sys, g.Path)

			// --- Inline random playtime selection ---
			playTime := minTime
			if minTime != maxTime {
				playTime = rand.Intn(maxTime-minTime+1) + minTime
			}
			// ----------------------------------------

			time.Sleep(time.Duration(playTime) * time.Second)
		}
	}
}

// Keep this one tiny helper because it’s genuinely shared logic
// (filter rules are reused elsewhere too)
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
