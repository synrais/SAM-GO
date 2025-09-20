package input

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	"github.com/synrais/SAM-GO/pkg/utils"
)

// GameIndex holds all indexed game entries for search.
var GameIndex []GameEntry

// GameEntry is one normalized entry in the index.
type GameEntry struct {
	Name string // normalized name
	Ext  string // extension (smd, nes, zip, …)
	Path string // original line from Search.txt
}

// SearchAndPlay enters search mode.
func SearchAndPlay() {
	// Enter search state
	utils.SetState(utils.StateAttractPaused)

	fmt.Println("[SEARCH] 🔍 Entered search mode (Attract paused).")
	fmt.Println("[SEARCH] Type to filter, ENTER to launch, ESC to exit.")
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
			key := strings.ToUpper(m[1])

			switch key {
			case "SPACE":
				sb.WriteRune(' ')
				fmt.Printf("[SEARCH] Current query: %q\n", sb.String())

			case "ENTER":
				qn, qext := utils.NormalizeEntry(sb.String())
				if qn != "" {
					fmt.Printf("[SEARCH] Looking for: %q\n", sb.String())
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

			case "ESC":
				fmt.Println("[SEARCH] Exiting search mode")
				utils.SetState(utils.StateAttract) // back to attract
				return

			case "BACKSPACE":
				s := sb.String()
				if len(s) > 0 {
					sb.Reset()
					sb.WriteString(s[:len(s)-1])
				}
				fmt.Printf("[SEARCH] Current query: %q\n", sb.String())

			case "LEFT":
				if len(candidates) > 0 && idx > 0 {
					idx--
					fmt.Printf("[SEARCH] ▶ Launching (prev): %s\n", candidates[idx])
					launchGame(candidates[idx])
				}

			case "RIGHT":
				if len(candidates) > 0 && idx < len(candidates)-1 {
					idx++
					fmt.Printf("[SEARCH] ▶ Launching (next): %s\n", candidates[idx])
					launchGame(candidates[idx])
				}
			}
		}

		// Regular text input
		l := re.ReplaceAllString(line, "")
		for _, r := range l {
			if r == '\n' || r == '\r' {
				continue
			}
			sb.WriteRune(r)
			fmt.Printf("[SEARCH] Current query: %q\n", sb.String())
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
		} else if levenshtein(qn, e.Name) <= 3 {
			fuzzy = append(fuzzy, e.Path)
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
