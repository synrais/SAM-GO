//go:build linux

package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rivo/tview"
)

// Path to SAM gamelists on MiSTer
const gamelistRoot = "/media/fat/Scripts/.MiSTer_SAM/SAM_Gamelists"

func loadSystems() ([]string, error) {
	files, err := filepath.Glob(filepath.Join(gamelistRoot, "*_gamelist.txt"))
	if err != nil {
		return nil, err
	}
	var systems []string
	for _, f := range files {
		base := filepath.Base(f)
		system := strings.TrimSuffix(base, "_gamelist.txt")
		systems = append(systems, system)
	}
	return systems, nil
}

func loadGames(system string) ([][2]string, error) {
	path := filepath.Join(gamelistRoot, system+"_gamelist.txt")
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var games [][2]string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name := filepath.Base(line)
		name = strings.TrimSuffix(name, filepath.Ext(name))
		games = append(games, [2]string{name, line})
	}
	return games, scanner.Err()
}

func runGame(path string) {
	// Replace with actual SAM runner call
	fmt.Printf("Would run game: %s\n", path)
	// Example:
	// exec.Command("/media/fat/Scripts/.MiSTer_SAM/SAM", "-run", path).Run()
}

func main() {
	app := tview.NewApplication()

	systems, err := loadSystems()
	if err != nil || len(systems) == 0 {
		fmt.Println("No gamelists found:", err)
		os.Exit(1)
	}

	systemList := tview.NewList().
		ShowSecondaryText(false).
		SetBorder(true).
		SetTitle(" Systems ")

	gameList := tview.NewList().
		ShowSecondaryText(false).
		SetBorder(true).
		SetTitle(" Games ")

	// Populate system list
	for _, sys := range systems {
		sys := sys
		systemList.AddItem(sys, "", 0, func() {
			gameList.Clear()
			games, err := loadGames(sys)
			if err != nil {
				gameList.AddItem("Error loading games", "", 0, nil)
				return
			}
			for _, g := range games {
				name := g[0]
				path := g[1]
				gameList.AddItem(name, "", 0, func() {
					runGame(path)
				})
			}
		})
	}

	// Layout: system list left, game list right
	flex := tview.NewFlex().
		AddItem(systemList, 0, 1, true).
		AddItem(gameList, 0, 2, false)

	if err := app.SetRoot(flex, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}
