package attract

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/synrais/SAM-GO/pkg/utils"
)

// SearchAndPlay enters search mode and launches matching games.
func SearchAndPlay(inputCh <-chan string) {
	fmt.Println("[SEARCH] 🔍 Entered search mode (Attract paused).")
	fmt.Println("[SEARCH] Type to filter, ENTER to launch, ESC to exit.")

	// flatten index map to slice of paths
	allIndex := GetAllIndex()
	var index []string
	for _, paths := range allIndex {
		index = append(index, paths...)
	}
	fmt.Printf("[SEARCH] GameIndex loaded: %d entries\n", len(index))

	var sb strings.Builder
	var candidates []string
	idx := -1

	// get input map
	inputMap := SearchInputMap(&sb, &candidates, &idx, index, inputCh)

	for ev := range inputCh {
		evLower := strings.ToLower(ev)
		if action, ok := inputMap[evLower]; ok {
			action()
			if evLower == "esc" {
				// exit search mode cleanly
				return
			}
		} else {
			// handle single-character inputs like a–z, 0–9, punctuation
			if len(ev) == 1 {
				sb.WriteString(ev)
				fmt.Printf("[SEARCH] Current query: %q\n", sb.String())
			}
		}
	}
}

// --- Matching ---

func findMatches(qn, qext string, index []string) []string {
	var prefix, substring, fuzzy []string

	for _, path := range index {
		name, ext := utils.NormalizeEntry(path)

		if qext != "" && qext != ext {
			continue
		}

		if strings.HasPrefix(name, qn) {
			prefix = append(prefix, path)
		} else if strings.Contains(name, qn) {
			substring = append(substring, path)
		} else {
			if levenshtein(qn, name) <= 3 {
				fuzzy = append(fuzzy, path)
			}
		}
	}

	// sort each group
	sort.Slice(prefix, func(i, j int) bool { return prefix[i] < prefix[j] })
	sort.Slice(substring, func(i, j int) bool { return substring[i] < substring[j] })
	sort.Slice(fuzzy, func(i, j int) bool { return fuzzy[i] < fuzzy[j] })

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
	name, ext := utils.NormalizeEntry(path)
	systemID := DetectSystemFromPath(path) // you'll need a helper or map for this
	fmt.Printf("[SEARCH] %s %s %s <%s>\n",
		time.Now().Format("15:04:05"),
		systemID,
		strings.TrimSuffix(name, ext),
		path,
	)
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
