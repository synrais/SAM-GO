package input

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/utils"
)

// --- Search state ---

// GameIndex holds all indexed game entries for search.
var GameIndex []GameEntry

// GameEntry is one normalized entry in the index.
type GameEntry struct {
	Name string // normalized name
	Ext  string // extension (smd, nes, zip, …)
	Path string // original line from Search.txt
}

// SearchAndPlay enters search mode and blocks until ESC is pressed.
// This function handles keyboard input via RelayInputs → StreamEvents.
func SearchAndPlay(cfg *config.UserConfig) {
	fmt.Println("[SEARCH] 🔍 Entered search mode.")
	fmt.Println("[SEARCH] Type to filter, ENTER to launch, ESC to exit.")
	fmt.Printf("[SEARCH] GameIndex loaded: %d entries\n", len(GameIndex))

	events := StreamEvents(cfg)

	var sb strings.Builder
	var candidates []string
	idx := -1

	for ev := range events {
		key := string(ev)

		switch key {
		// --- Control Keys ---
		case "`":
			// Allow restarting query if backtick is pressed
			fmt.Println("[SEARCH] Restarting query…")
			sb.Reset()
			candidates = nil
			idx = -1

		case "space":
			sb.WriteRune(' ')
			fmt.Printf("[SEARCH] Current query: %q\n", sb.String())

		case "enter":
			qn, qext := utils.NormalizeEntry(sb.String())
			if qext != "" {
				fmt.Printf("[SEARCH] Looking for: %q (.%s)\n", sb.String(), qext)
			} else {
				fmt.Printf("[SEARCH] Looking for: %q\n", sb.String())
			}

			if qn != "" {
				fmt.Printf("[SEARCH] Searching... (%d titles)\n", len(GameIndex))
				candidates = findMatches(qn, qext)
				if len(candidates) > 0 {
					idx = 0
					fmt.Printf("[SEARCH] ▶ Launching: %s\n", candidates[idx])
					launchGame(candidates[idx])
				} else {
					fmt.Println("[SEARCH] No match found")
				}
			}
			sb.Reset()
			fmt.Println("[SEARCH] Ready. Use ←/→ to browse, ESC to exit.")

		case "esc", "escape":
			fmt.Println("[SEARCH] Exiting search mode.")
			return

		case "backspace":
			s := sb.String()
			if len(s) > 0 {
				sb.Reset()
				sb.WriteString(s[:len(s)-1])
			}
			fmt.Printf("[SEARCH] Current query: %q\n", sb.String())

		case "left":
			if len(candidates) > 0 && idx > 0 {
				idx--
				fmt.Printf("[SEARCH] ▶ Launching (prev): %s\n", candidates[idx])
				launchGame(candidates[idx])
			}

		case "right":
			if len(candidates) > 0 && idx < len(candidates)-1 {
				idx++
				fmt.Printf("[SEARCH] ▶ Launching (next): %s\n", candidates[idx])
				launchGame(candidates[idx])
			}

		// --- Text Input (normal characters) ---
		default:
			if len(key) == 1 { // single char
				sb.WriteString(key)
				fmt.Printf("[SEARCH] Current query: %q\n", sb.String())
			}
		}
	}
}

// --- Matching ---

func findMatches(qn, qext string) []string {
	var prefix, substring, fuzzy []string

	for _, e := range GameIndex {
		if strings.HasPrefix(e.Name, "# SYSTEM:") {
			continue
		}
		if qext != "" && qext != e.Ext {
			continue
		}
		if strings.HasPrefix(e.Name, qn) {
			prefix = append(prefix, e.Path)
		} else if strings.Contains(e.Name, qn) {
			substring = append(substring, e.Path)
		} else {
			dist := levenshtein(qn, e.Name)
			if dist <= 3 {
				fuzzy = append(fuzzy, e.Path)
			}
		}
	}

	// Sort for consistency
	sort.Strings(prefix)
	sort.Strings(substring)
	sort.Strings(fuzzy)

	out := append(prefix, substring...)
	out = append(out, fuzzy...)

	if len(out) > 200 {
		out = out[:200]
	}

	fmt.Printf("[SEARCH] Matches found: %d\n", len(out))
	return out
}

// --- Helpers ---

func launchGame(path string) {
	exe, err := os.Executable()
	if err != nil {
		fmt.Println("[SEARCH] ERROR: could not resolve executable for launch")
		return
	}
	fmt.Printf("[SEARCH] Exec: %s -run %q\n", exe, path)
	cmd := exec.Command(exe, "-run", path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Start()
}

func levenshtein(a, b string) int {
	la := len(a)
	lb := len(b)
	d := make([][]int, la+1)
	for i := range d {
		d[i] = make([]int, lb+1)
	}
	for i := 0; i <= la; i++ {
		d[i][0] = i
	}
	for j := 0; j <= lb; j++ {
		d[0][j] = j
	}
	for i := 1; i <= la; i++ {
		for j := 1; j <= lb; j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			d[i][j] = min(d[i-1][j]+1, min(d[i][j-1]+1, d[i-1][j-1]+cost))
		}
	}
	return d[la][lb]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
