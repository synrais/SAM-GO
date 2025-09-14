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
	lists = make(map[string][]string)
)

// ReloadAll clears and reloads all .txt files from a directory into RAM.
func ReloadAll(dir string) error {
	mu.Lock()
	defer mu.Unlock()

	lists = make(map[string][]string)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reload cache: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".txt") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		lines, err := readLines(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[WARN] failed to read %s: %v\n", path, err)
			continue
		}
		lists[e.Name()] = lines
	}

	return nil
}

// GetList returns the cached lines for a filename (e.g. "Search.txt").
func GetList(name string) []string {
	mu.RLock()
	defer mu.RUnlock()
	return append([]string(nil), lists[name]...) // defensive copy
}

// readLines helper
func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			out = append(out, line)
		}
	}
	return out, scanner.Err()
}
