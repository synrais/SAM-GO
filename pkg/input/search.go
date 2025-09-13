package input

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
)

var searching atomic.Bool

// GameEntry is one normalized entry in the index.
type GameEntry struct {
	Name string // normalized name (alnum only, lowercased, no brackets)
	Ext  string // extension (smd, nes, zip, …)
	Path string // original path line from gamelist
}

var (
	gameIndex  []GameEntry
	indexBuilt atomic.Bool
)

// IsSearching reports whether search mode is active.
func IsSearching() bool {
	return searching.Load()
}

// SearchAndPlay enters search mode: type a query, press Enter to search,
// Left/Right cycle through results, Backspace always edits buffer, Escape exits.
func SearchAndPlay() {
	fmt.Println("Attract mode paused")
	fmt.Println("Search: type your game and press Enter")

	searching.Store(true)
	defer func() {
		searching.Store(false)
		fmt.Println("Attract mode resumed")
	}()

	ch := StreamKeyboards()
	re := regexp.MustCompile(`<([^>]+)>`)

	var sb strings.Builder
	var candidates []string
	idx := 0

	for line := range ch {
		l := strings.ToLower(line)
		matches := re.FindAllStringSubmatch(l, -1)

		for _, m := range matches {
			switch m[1] {
			case "enter":
				fmt.Printf("[ENTER pressed] Raw query: %q\n", sb.String())
				qn, qext := normalizeQuery(sb.String())
				fmt.Printf("[NORMALIZED] Query=%q Ext=%q\n", qn, qext)
				if qn != "" {
					candidates = findMatches(qn, qext)
					fmt.Printf("[MATCHES] Found %d candidates for query %q\n", len(candidates), qn)
					for i, c := range candidates {
						fmt.Printf("  %d: %s\n", i, c)
					}
					if len(candidates) > 0 {
						idx = 0
						fmt.Printf("[LAUNCH] Launching: %s\n", candidates[idx])
						launchGame(candidates[idx])
					} else {
						fmt.Println("[NO MATCH] No match found")
					}
				}
				return // exit search mode
			case "escape":
				fmt.Println("[ESC pressed] Exiting search mode")
				return // exit search mode
			case "backspace":
				s := sb.String()
				if len(s) > 0 {
					sb.Reset()
					sb.WriteString(s[:len(s)-1])
				}
				fmt.Printf("[BACKSPACE] Buffer now: %q\n", sb.String())
			case "left":
				if len(candidates) > 0 && idx > 0 {
					idx--
					fmt.Printf("[LEFT] Switching to: %s\n", candidates[idx])
					launchGame(candidates[idx])
				}
			case "right":
				if len(candidates) > 0 && idx < len(candidates)-1 {
					idx++
					fmt.Printf("[RIGHT] Switching to: %s\n", candidates[idx])
					launchGame(candidates[idx])
				}
			}
		}

		// Regular text always updates the buffer
		l = re.ReplaceAllString(l, "")
		for _, r := range l {
			if r == '\n' || r == '\r' {
				continue
			}
			sb.WriteRune(r)
			fmt.Printf("[CHAR] Buffer now: %q\n", sb.String())
		}
	}
}

// --- Index building ---

func ensureIndex() {
	if !indexBuilt.Load() {
		buildIndex()
		indexBuilt.Store(true)
	}
}

func buildIndex() {
	files, _ := filepath.Glob("/media/fat/Scripts/.MiSTer_SAM/SAM_Gamelists/*_gamelist.txt")
	if len(files) == 0 {
		fmt.Println("[DEBUG] No gamelist files found")
		return
	}

	for _, f := range files {
		fmt.Printf("[DEBUG] Indexing file: %s\n", f)
		file, err := os.Open(f)
		if err != nil {
			fmt.Printf("[ERROR] Failed to open %s: %v\n", f, err)
			continue
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(stripTimestamp(scanner.Text()))
			if line == "" {
				continue
			}
			name, ext := normalizeEntry(line)
			gameIndex = append(gameIndex, GameEntry{
				Name: name,
				Ext:  ext,
				Path: line,
			})
		}
		file.Close()
	}

	fmt.Printf("[DEBUG] Indexed %d entries\n", len(gameIndex))
}

// --- Matching ---

func findMatches(qn, qext string) []string {
	ensureIndex()

	type cand struct {
		path string
		dist int
	}
	var list []cand

	for _, e := range gameIndex {
		if qext != "" && qext != e.Ext {
			continue
		}
		dist := levenshtein(qn, e.Name)
		list = append(list, cand{path: e.Path, dist: dist})
	}

	sort.Slice(list, func(i, j int) bool { return list[i].dist < list[j].dist })

	out := make([]string, len(list))
	for i, c := range list {
		out[i] = c.path
	}
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

func stripTimestamp(line string) string {
	if strings.HasPrefix(line, "<") {
		if idx := strings.Index(line, ">"); idx > 1 {
			return line[idx+1:]
		}
	}
	return line
}

var (
	nonAlnum     = regexp.MustCompile(`[^a-z0-9]+`)
	bracketChars = regexp.MustCompile(`(\([^)]*\)|\[[^]]*\])`)
)

func normalizeQuery(q string) (string, string) {
	base := filepath.Base(q)
	ext := strings.ToLower(filepath.Ext(base))
	name := strings.TrimSuffix(base, ext)
	name = bracketChars.ReplaceAllString(name, "")
	name = strings.ToLower(name)
	name = nonAlnum.ReplaceAllString(name, "")
	return name, strings.TrimPrefix(ext, ".")
}

func normalizeEntry(p string) (string, string) {
	base := filepath.Base(p)
	ext := strings.ToLower(filepath.Ext(base))
	name := strings.TrimSuffix(base, ext)
	name = bracketChars.ReplaceAllString(name, "")
	name = strings.ToLower(name)
	name = nonAlnum.ReplaceAllString(name, "")
	return name, strings.TrimPrefix(ext, ".")
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
