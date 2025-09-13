package input

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
)

var searching atomic.Bool

// SearchAndPlay waits for keyboard input to build a search string and then
// launches the closest matching game from the generated gamelists.
func SearchAndPlay() {
	fmt.Println("Search: type your game and press Enter")

	searching.Store(true)
	defer searching.Store(false)

	ch := StreamKeyboards()
	re := regexp.MustCompile(`<([^>]+)>`)
	var sb strings.Builder

	for line := range ch {
		l := strings.ToLower(line)
		matches := re.FindAllStringSubmatch(l, -1)
		for _, m := range matches {
			switch m[1] {
			case "enter":
				query := sb.String()
				if query != "" {
					if path := findBestMatch(query); path != "" {
						launchGame(path)
					} else {
						fmt.Println("No match found for", query)
					}
				}
				return
			case "escape":
				return
			case "backspace":
				s := sb.String()
				if len(s) > 0 {
					sb.Reset()
					sb.WriteString(s[:len(s)-1])
				}
			}
		}
		l = re.ReplaceAllString(l, "")
		for _, r := range l {
			if r == '\n' || r == '\r' {
				continue
			}
			sb.WriteRune(r)
		}
	}
}

func findBestMatch(query string) string {
	files, _ := filepath.Glob("/tmp/.SAM_List/*_gamelist.txt")
	if len(files) == 0 {
		return ""
	}
	qn, qext := normalizeQuery(query)
	bestPath := ""
	bestDist := -1
	for _, f := range files {
		file, err := os.Open(f)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(stripTimestamp(scanner.Text()))
			if line == "" {
				continue
			}
			name, ext := normalizeEntry(line)
			if qext != "" && qext != ext {
				continue
			}
			if name == qn {
				file.Close()
				return line
			}
			dist := levenshtein(qn, name)
			if bestDist == -1 || dist < bestDist {
				bestDist = dist
				bestPath = line
			}
		}
		file.Close()
	}
	return bestPath
}

func launchGame(path string) {
	exe, err := os.Executable()
	if err != nil {
		return
	}
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
