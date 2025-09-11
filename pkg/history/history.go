package history

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/games"
	"github.com/synrais/SAM-GO/pkg/run"
)

const historyFile = "/tmp/.SAM_History.txt"
const listDir = "/tmp/.SAM_List"

var position = -1

func loadHistory() []string {
	file, err := os.Open(historyFile)
	if err != nil {
		return nil
	}
	defer file.Close()
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}
	if position == -1 || position > len(lines) {
		position = len(lines)
	}
	return lines
}

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

func appendLine(path, line string) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(line + "\n")
}

func play(path string) error {
	system, _ := games.BestSystemMatch(&config.UserConfig{}, path)
	gameName := filepath.Base(path)
	content := fmt.Sprintf("[%s] %s", system.Id, gameName)
	_ = os.WriteFile("/tmp/Now_Playing.txt", []byte(content), 0644)
	return run.Run([]string{path})
}

// Print outputs the history to stdout.
func Print() {
	lines := loadHistory()
	for i, l := range lines {
		fmt.Printf("%d: %s\n", i, l)
	}
}

// Back runs the previous game in history.
func Back() error {
	lines := loadHistory()
	if len(lines) == 0 {
		return fmt.Errorf("history empty")
	}
	if position > len(lines) {
		position = len(lines)
	}
	if position <= 0 {
		position = 0
		return play(lines[position])
	}
	position--
	return play(lines[position])
}

// Forward runs the next game in history or a random one if exhausted.
func Forward() error {
	lines := loadHistory()
	if position == -1 {
		position = len(lines)
	}
	if position >= len(lines)-1 {
		return playRandom()
	}
	position++
	return play(lines[position])
}

func playRandom() error {
	files, err := filepath.Glob(filepath.Join(listDir, "*_gamelist.txt"))
	if err != nil || len(files) == 0 {
		return fmt.Errorf("no gamelists found")
	}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	listFile := files[r.Intn(len(files))]
	lines, err := readLines(listFile)
	if err != nil || len(lines) == 0 {
		return fmt.Errorf("empty gamelist: %s", listFile)
	}
	gamePath := lines[r.Intn(len(lines))]
	systemID := strings.TrimSuffix(filepath.Base(listFile), "_gamelist.txt")
	gameName := filepath.Base(gamePath)
	_ = os.WriteFile("/tmp/Now_Playing.txt", []byte(fmt.Sprintf("[%s] %s", systemID, gameName)), 0644)
	appendLine(historyFile, gamePath)
	return run.Run([]string{gamePath})
}

// Reset sets history pointer to the end.
func Reset() {
	lines := loadHistory()
	position = len(lines)
}
