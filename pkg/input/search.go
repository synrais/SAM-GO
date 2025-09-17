package input

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/synrais/SAM-GO/pkg/utils"
)

var searching atomic.Bool

// --- Search state ---
// GameIndex holds all indexed game entries for search.
var GameIndex []GameEntry

// GameEntry is one normalized entry in the index.
type GameEntry struct {
	Name string // normalized name
	Ext  string // extension (smd, nes, zip, …)
	Path string // original line from Search.txt
}

// IsSearching reports whether search mode is active.
func IsSearching() bool {
	return searching.Load()
}

// SearchAndPlay enters search mode.
func SearchAndPlay() {
	fmt.Println("Attract mode paused, press ESC to resume")
	fmt.Println("Search: type your game and press Enter")

	searching.Store(true)
	defer searching.Store(false)

	// 🟢 Debug: show how many games are indexed
	fmt.Printf("[SEARCH] GameIndex loaded: %d entries\n", len(GameIndex))

	ch := StreamKeyboards()
	re := regexp.MustCompile(`<([^>]+)>`)

	var sb strings.Builder
	var candidates []string
	idx := -1

	for line := range ch {
		// Look for <TOKENS>
		matches := re.FindAllStringSubmatch(line, -1)

		for _, m := range matches {
			key := strings.ToUpper(m[1]) // force uppercase for all special keys

			switch key {
			case "SPACE":
				sb.WriteRune(' ')
				fmt.Printf("[SPACE] Search: %q\n", sb.String())

			case "ENTER":
				qn, qext := utils.NormalizeEntry(sb.String())

				// 🟢 Debug: show the normalized query
				fmt.Printf("[SEARCH] Query raw=%q normalized=%q ext=%q\n", sb.String(), qn, qext)

				if qn != "" {
					fmt.Printf("[SEARCH] Searching... (%d titles)\n", len(GameIndex))
					candidates = findMatches(qn, qext)
					if len(candidates) > 0 {
						idx = 0
						fmt.Printf("[ENTER] Launching: %s\n", candidates[idx])
						launchGame(candidates[idx])
					} else {
						fmt.Println("[NO MATCH] No match found")
					}
				}
				// Reset buffer after enter
				sb.Reset()
				fmt.Println("Search complete. Left Right keys to browse results.")
				fmt.Println("Search again or ESC to resume attract")

			case "ESC":
				fmt.Println("[ESC] Exiting search mode")
				searching.Store(false)
				fmt.Println("Attract mode resumed")
				return

			case "BACKSPACE":
				s := sb.String()
				if len(s) > 0 {
					sb.Reset()
					sb.WriteString(s[:len(s)-1])
				}
				fmt.Printf("[BACKSPACE] Search: %q\n", sb.String())

			case "LEFT":
				if len(candidates) > 0 && idx > 0 {
					idx--
					fmt.Printf("[LEFT] Launching: %s\n", candidates[idx])
					launchGame(candidates[idx])
				}

			case "RIGHT":
				if len(candidates) > 0 && idx < len(candidates)-1 {
					idx++
					fmt.Printf("[RIGHT] Launching: %s\n", candidates[idx])
					launchGame(candidates[idx])
				}
			}
		}

		// Regular text input goes into buffer
		l := re.ReplaceAllString(line, "")
		for _, r := range l {
			if r == '\n' || r == '\r' {
				continue
			}
			// allow spaces into buffer
			sb.WriteRune(r)
			fmt.Printf("[CHAR] Search: %q\n", sb.String())
		}
	}
}

// --- Matching ---

func findMatches(qn, qext string) []string {
	var prefix, substring, fuzzy []string

	for _, e := range GameIndex {
		// Skip entries that are separators (e.g., # SYSTEM: systemId #)
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
			if dist <= 3 { // only allow close fuzzy matches
				fuzzy = append(fuzzy, e.Path)
			}
		}
	}

	// Sort for consistency
	sort.Strings(prefix)
	sort.Strings(substring)
	sort.Strings(fuzzy)

	// Concatenate with priority
	out := append(prefix, substring...)
	out = append(out, fuzzy...)

	// Restrict to top 200
	if len(out) > 200 {
		out = out[:200]
	}

	// Show just summary info
	fmt.Printf("[SEARCH] Matches found: %d\n", len(out))

	return out
}

// --- Helpers ---

func launchGame(path string) {
	exe, err := os.Executable()
	if err != nil {
		fmt.Println("[ERROR] Could not resolve executable for launch")
		return
	}
	fmt.Printf("[EXEC] %s -run %q\n", exe, path)
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
