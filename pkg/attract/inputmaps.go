package attract

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/synrais/SAM-GO/pkg/config"
)

// -----------------------------
// History navigation + Timer reset
// -----------------------------

var currentIndex int = -1

// resetTimer safely stops and resets a timer, ignoring nil.
func resetTimer(timer *time.Timer, d time.Duration) {
	if timer == nil {
		return
	}
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(d)
}

// Next moves forward in history if possible, otherwise picks a random game.
// Always runs the game and resets timer.
func Next(timer *time.Timer, cfg *config.UserConfig, r *rand.Rand) (string, bool) {
	hist := GetList("History.txt")

	// Forward in history
	if currentIndex >= 0 && currentIndex < len(hist)-1 {
		currentIndex++
		path := hist[currentIndex]
		Run([]string{path})
		resetTimer(timer, ParsePlayTime(cfg.Attract.PlayTime, r))
		return path, true
	}

	// Otherwise pick a fresh random game
	path := PickRandomGame(cfg, r)
	if path == "" {
		fmt.Println("[Attract] No game available to play.")
		return "", false
	}

	hist = append(hist, path)
	SetList("History.txt", hist)
	currentIndex = len(hist) - 1

	Run([]string{path})
	resetTimer(timer, ParsePlayTime(cfg.Attract.PlayTime, r))
	return path, true
}

// Back moves backward in history, runs game, resets timer.
func Back(timer *time.Timer, cfg *config.UserConfig, r *rand.Rand) (string, bool) {
	hist := GetList("History.txt")
	if currentIndex > 0 {
		currentIndex--
		path := hist[currentIndex]
		Run([]string{path})
		resetTimer(timer, ParsePlayTime(cfg.Attract.PlayTime, r))
		return path, true
	}
	return "", false
}

// Aliases for consistency
func PlayNext(timer *time.Timer, cfg *config.UserConfig, r *rand.Rand) (string, bool) {
	return Next(timer, cfg, r)
}

func PlayBack(timer *time.Timer, cfg *config.UserConfig, r *rand.Rand) (string, bool) {
	return Back(timer, cfg, r)
}
