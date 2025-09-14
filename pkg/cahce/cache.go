package cache

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	mu    sync.RWMutex
	lists = make(map[string][]string) // filename → lines
)

// ReloadAll preloads all lists from tmpDir (e.g., /tmp/.SAM_List) into memory.
func ReloadAll(tmpDir string) error {
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		return fmt.Errorf("cache reload failed: %w", err)
	}

	tmp := make(map[string][]string)

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		path := filepath.Join(tmpDir, f.Name())
		lines, err := readLines(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[WARN] Could not read %s: %v\n", f.Name(), err)
			continue
		}
		tmp[f.Name()] = lines
	}

	mu.Lock()
	lists = tmp
	mu.Unlock()

	fmt.Printf("[CACHE] Preloaded %d lists into RAM\n", len(lists))
	return nil
}

// GetList returns a list by filename (e.g., "Search.txt").
func GetList(name string) []string {
	mu.RLock()
	defer mu.RUnlock()
	return clone(lists[name])
}

// GetSystemList returns <system>_gamelist.txt if available.
func GetSystemList(systemId string) []string {
	return GetList(systemId + "_gamelist.txt")
}

// GetMasterlist returns Masterlist.txt if available.
func GetMasterlist() []string {
	return GetList("Masterlist.txt")
}

// --- helpers ---

func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines, scanner.Err()
}

func clone(in []string) []string {
	if in == nil {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}
