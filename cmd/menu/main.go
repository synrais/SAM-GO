package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gdamore/tcell/v2"
	"github.com/synrais/SAM-GO/pkg/attract"
)

type MenuItem struct {
	System string
	Path   string
	IsSystem bool
	Expanded bool
}

func buildMenu(master []string) []MenuItem {
	var items []MenuItem
	var currentSystem string

	for _, line := range master {
		if len(line) > 9 && line[:9] == "# SYSTEM:" {
			currentSystem = line[9:]
			items = append(items, MenuItem{System: currentSystem, IsSystem: true})
		} else if line != "" {
			items = append(items, MenuItem{System: currentSystem, Path: line})
		}
	}
	return items
}

func main() {
	// Load the in-memory Masterlist slice
	master := attract.GetMasterlist() // <- you’ll need to expose this accessor

	items := buildMenu(master)

	// Init tcell screen
	s, err := tcell.NewScreen()
	if err != nil {
		log.Fatalf("tcell.NewScreen: %v", err)
	}
	if err = s.Init(); err != nil {
		log.Fatalf("s.Init: %v", err)
	}
	defer s.Fini()

	s.Clear()
	style := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorBlack)

	selected := 0

	// Render function
	draw := func() {
		s.Clear()
		y := 0
		for i, item := range items {
			line := ""
			if item.IsSystem {
				prefix := "+"
				if item.Expanded {
					prefix = "-"
				}
				line = fmt.Sprintf("[%s] %s", prefix, item.System)
			} else if item.Expanded || items[i-1].Expanded {
				line = "    " + item.Path
			} else {
				continue
			}

			lineStyle := style
			if i == selected {
				lineStyle = lineStyle.Reverse(true)
			}
			for x, r := range line {
				s.SetContent(x, y, r, nil, lineStyle)
			}
			y++
		}
		s.Show()
	}

	// Main loop
	for {
		draw()
		ev := s.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			switch ev.Key() {
			case tcell.KeyEscape:
				return
			case tcell.KeyUp:
				if selected > 0 {
					selected--
				}
			case tcell.KeyDown:
				if selected < len(items)-1 {
					selected++
				}
			case tcell.KeyEnter:
				if items[selected].IsSystem {
					items[selected].Expanded = !items[selected].Expanded
				}
			}
		case *tcell.EventResize:
			s.Sync()
		}
	}
}
