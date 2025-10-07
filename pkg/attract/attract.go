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

// StartAttractMode picks and plays random games endlessly using the existing menu database.
func StartAttractMode(userCfg *config.UserConfig, files []gamesdb.FileInfo) error {
	fmt.Println("=== Starting Attract Mode ===")

	cfg, err := config.LoadINI()
	if err != nil {
		return fmt.Errorf("failed to load attract config: %w", err)
	}

	filtered := filterSystems(files, cfg)
	if len(filtered) == 0 {
		return fmt.Errorf("no games available after filtering")
	}

	// inline playtime parser
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

	rand.Seed(time.Now().UnixNano())

	for {
		// choose a completely random game each iteration
		game := filtered[rand.Intn(len(filtered))]

		sys, err := games.GetSystem(game.SystemId)
		if err != nil || sys == nil {
			continue
		}

		display := game.Name
		if game.Ext != "" {
			display += "." + game.Ext
		}
		fmt.Printf("[Attract] Launching %s (%s)\n", display, sys.Name)

		if err := mister.LaunchGame(userCfg, *sys, game.Path); err != nil {
			fmt.Printf("[Attract] failed to launch %s: %v\n", display, err)
			continue
		}

		playTime := minTime
		if minTime != maxTime {
			playTime = rand.Intn(maxTime-minTime+1) + minTime
		}
		time.Sleep(time.Duration(playTime) * time.Second)
	}
}

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
