package attract

import (
	"fmt"
	"sort"
	"strings"

	"github.com/synrais/SAM-GO/pkg/utils"
)

// SearchAndPlay enters search mode and launches matching games.
func SearchAndPlay(inputCh <-chan string) {
	fmt.Println("[SEARCH] 🔍 Entered search mode (Attract paused).")
	fmt.Println("[SEARCH] Type to filter, ENTER to launch, ESC to exit.")

	index := GetGameIndex()
	fmt.Printf("[SEARCH] GameIndex loaded: %d entries\n", len(index))

	var sb strings.Builder
	var candidates []string
	idx := -1

	for ev := range inputCh {
		switch strings.ToLower(ev) {
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
				fmt.Printf("[SEARCH] Searching... (%d titles)\n", len(index))
				candidates = findMatches(qn, qext, index)
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

		case "esc":
			fmt.Println("[SEARCH] Exiting search mode (Attract resumed).")
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

		default:
			// treat as regular character
			if len(ev) == 1 {
				sb.WriteString(ev)
				fmt.Printf("[SEARCH] Current query: %q\n", sb.String())
			}
		}
	}
}

// --- Matching ---

func findMatches(qn, qext string, index []GameEntry) []string {
	var prefix, substring, fuzzy []string

	for _, e := range index {
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
			if levenshtein(qn, e.Name) <= 3 {
				fuzzy = append(fuzzy, e.Path)
			}
		}
	}

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
	fmt.Printf("[SEARCH] ▶ Running game: %s\n", path)
	Run([]string{path})
}

func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
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
