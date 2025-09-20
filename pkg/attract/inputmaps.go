package attract

import (
	"fmt"
	"os"
	"strings"
	"time"
	"math/rand"

	"github.com/synrais/SAM-GO/pkg/config"
)

// Generic function type for mapped inputs
type InputAction func()

// --- Attract Mode Input Map ---
func AttractInputMap(cfg *config.UserConfig, r *rand.Rand, timer *time.Timer, inputCh <-chan string) map[string]InputAction {
	return map[string]InputAction{
		"esc": func() {
			fmt.Println("[Attract] Exiting attract mode.")
			os.Exit(0)
		},
		"space": func() {
			fmt.Println("[Attract] Skipped current game.")
			timer.Stop()
		},
		"`": func() {
			fmt.Println("[Attract] Entering search mode...")
			SearchAndPlay(inputCh)
			fmt.Println("[Attract] Resuming attract mode.")
			resetTimer(timer, ParsePlayTime(cfg.Attract.PlayTime, r))
		},
		"right": func() {
			if next, ok := Next(); ok {
				fmt.Println("[Attract] Going forward in history.")
				Run([]string{next})
				resetTimer(timer, ParsePlayTime(cfg.Attract.PlayTime, r))
			} else {
				fmt.Println("[Attract] Skipped current game.")
				timer.Stop()
			}
		},
		"button1": func() { // alias for right
			if next, ok := Next(); ok {
				fmt.Println("[Attract] Going forward in history.")
				Run([]string{next})
				resetTimer(timer, ParsePlayTime(cfg.Attract.PlayTime, r))
			}
		},
		"left": func() {
			if prev, ok := PlayBack(); ok {
				fmt.Println("[Attract] Going back in history.")
				Run([]string{prev})
				resetTimer(timer, ParsePlayTime(cfg.Attract.PlayTime, r))
			}
		},
		"button2": func() { // alias for left
			if prev, ok := PlayBack(); ok {
				fmt.Println("[Attract] Going back in history.")
				Run([]string{prev})
				resetTimer(timer, ParsePlayTime(cfg.Attract.PlayTime, r))
			}
		},
	}
}

// --- Search Mode Input Map ---
func SearchInputMap(sb *strings.Builder, candidates *[]GameEntry, idx *int, index []GameEntry, inputCh <-chan string) map[string]InputAction {
	return map[string]InputAction{
		"space": func() {
			sb.WriteRune(' ')
			fmt.Printf("[SEARCH] Current query: %q\n", sb.String())
		},
		"enter": func() {
			qn, qext := NormalizeQuery(sb.String())
			if qn != "" {
				fmt.Printf("[SEARCH] Searching... (%d titles)\n", len(index))
				*candidates = findMatches(qn, qext, index)
				if len(*candidates) > 0 {
					*idx = 0
					launchGame((*candidates)[*idx])
				} else {
					fmt.Println("[SEARCH] No match found")
				}
			}
			sb.Reset()
			fmt.Println("[SEARCH] Ready. Use ←/→ to browse, ESC to exit.")
		},
		"esc": func() {
			fmt.Println("[SEARCH] Exiting search mode (Attract resumed).")
			// handled by caller to break loop
		},
		"backspace": func() {
			s := sb.String()
			if len(s) > 0 {
				sb.Reset()
				sb.WriteString(s[:len(s)-1])
			}
			fmt.Printf("[SEARCH] Current query: %q\n", sb.String())
		},
		"left": func() {
			if len(*candidates) > 0 && *idx > 0 {
				*idx--
				launchGame((*candidates)[*idx])
			}
		},
		"right": func() {
			if len(*candidates) > 0 && *idx < len(*candidates)-1 {
				*idx++
				launchGame((*candidates)[*idx])
			}
		},
	}
}

// --- Helpers ---

func resetTimer(timer *time.Timer, dur time.Duration) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(dur)
}

// Normalize query with utils.NormalizeEntry
func NormalizeQuery(s string) (string, string) {
	qn, qext := utils.NormalizeEntry(s)
	if qext != "" {
		fmt.Printf("[SEARCH] Looking for: %q (.%s)\n", s, qext)
	} else {
		fmt.Printf("[SEARCH] Looking for: %q\n", s)
	}
	return qn, qext
}
