package attract

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/synrais/SAM-GO/pkg/utils"
)

// RemoveCache deletes a key from the chosen cache.
func RemoveCache(cacheType, key string) {
	mu.Lock()
	defer mu.Unlock()
	cache := cacheSelector(cacheType)
	if cache == nil {
		return
	}
	delete(cache, key)
}

// AmendCache appends values onto an existing cache entry.
func AmendCache(cacheType, key string, vals []string) {
	mu.Lock()
	defer mu.Unlock()
	cache := cacheSelector(cacheType)
	if cache == nil {
		return
	}
	cache[key] = append(cache[key], vals...)
}

// -----------------------------
// Core in-RAM caches
// -----------------------------

var (
	mu     sync.RWMutex
	lists  = make(map[string][]string) // gamelists per system (per file)
	master = make(map[string][]string) // master list per system
	index  = make(map[string][]string) // index per system
)

func cacheSelector(cacheType string) map[string][]string {
	switch cacheType {
	case "lists":
		return lists
	case "master":
		return master
	case "index":
		return index
	default:
		return nil
	}
}

// -----------------------------
// Reset / Load helpers
// -----------------------------

func ResetAll() {
	mu.Lock()
	defer mu.Unlock()
	lists = make(map[string][]string)
	master = make(map[string][]string)
	index = make(map[string][]string)
}

func ReloadAll(dir string) error {
	mu.Lock()
	defer mu.Unlock()

	lists = make(map[string][]string)
	master = make(map[string][]string)
	index = make(map[string][]string)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reload cache: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		path := filepath.Join(dir, e.Name())
		lines, err := utils.ReadLines(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[WARN] failed to read %s: %v\n", path, err)
			continue
		}

		switch e.Name() {
		case "MasterList":
			master["__all__"] = append([]string(nil), lines...)
		case "GameIndex":
			index["__all__"] = append([]string(nil), lines...)
		default:
			lists[e.Name()] = append([]string(nil), lines...)
		}
	}
	return nil
}

// -----------------------------
// Unified cache API
// -----------------------------

func GetCache(cacheType, key string) []string {
	mu.RLock()
	defer mu.RUnlock()
	cache := cacheSelector(cacheType)
	if cache == nil {
		return nil
	}
	return append([]string(nil), cache[key]...)
}

func SetCache(cacheType, key string, vals []string) {
	mu.Lock()
	defer mu.Unlock()
	cache := cacheSelector(cacheType)
	if cache == nil {
		return
	}
	cache[key] = append([]string(nil), vals...)
}

func CacheKeys(cacheType string) []string {
	mu.RLock()
	defer mu.RUnlock()
	cache := cacheSelector(cacheType)
	if cache == nil {
		return nil
	}
	keys := make([]string, 0, len(cache))
	for k := range cache {
		keys = append(keys, k)
	}
	return keys
}

func FlattenCache(cacheType string) []string {
	mu.RLock()
	defer mu.RUnlock()
	cache := cacheSelector(cacheType)
	if cache == nil {
		return nil
	}

	all := []string{}
	for sys, lines := range cache {
		all = append(all, "# SYSTEM: "+sys)
		all = append(all, lines...)
	}
	return all
}

func FlattenSystem(cacheType, systemID string) []string {
	mu.RLock()
	defer mu.RUnlock()
	cache := cacheSelector(cacheType)
	if cache == nil {
		return nil
	}
	return append([]string(nil), cache[systemID]...)
}

// -----------------------------
// IPC (Unix socket for menu access)
// -----------------------------

const socketPath = "/tmp/sam.sock"

func StartIPCServer() error {
	_ = os.Remove(socketPath)
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", socketPath, err)
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				continue
			}
			go handleConn(conn)
		}
	}()
	return nil
}

func handleConn(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, " ", 2)
		cmd := parts[0]
		arg := ""
		if len(parts) > 1 {
			arg = parts[1]
		}

		switch cmd {
		case "LIST_SYSTEMS":
			for _, sys := range CacheKeys("lists") {
				fmt.Fprintln(conn, sys)
			}
		case "LIST_GAMES":
			for _, g := range GetCache("lists", arg) {
				fmt.Fprintln(conn, g)
			}
		case "LIST_MASTER":
			for _, g := range FlattenSystem("master", arg) {
				fmt.Fprintln(conn, g)
			}
		case "LIST_INDEX":
			for _, g := range FlattenSystem("index", arg) {
				fmt.Fprintln(conn, g)
			}
		case "RUN_GAME":
			if arg == "" {
				fmt.Fprintln(conn, "ERR missing game path")
				continue
			}
			go Run([]string{arg})
			fmt.Fprintln(conn, "OK")
		default:
			fmt.Fprintln(conn, "ERR unknown command")
		}
	}
}

func IPCRequest(msg string) (string, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	if _, err := conn.Write([]byte(msg + "\n")); err != nil {
		return "", err
	}

	buf, err := io.ReadAll(conn)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}
