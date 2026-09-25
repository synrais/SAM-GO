package main

import (
	"strings"

	"github.com/synrais/SAM-GO/pkg/games"
)

// -------------------------
// Group Headers
// -------------------------
//
// Optional headings in the systems list, one above each first-level group
// (Display & Sorting -> Systems list sorting -> Group headers). They're
// plain ASCII so they show on any console font, and can't be selected.

var optHeaders = labelOption{"Group headers", []string{"Off", "Line", "Double", "Brackets", "Dots", "Minimal"}, 0}

// categoryTitles are the heading names for categories.
var categoryTitles = map[string]string{
	"Arcade": "Arcade", "Console": "Consoles", "Handheld": "Handhelds",
	"Computer": "Computers", "Other": "Other",
}

// groupHeader draws one heading, centred in width, e.g.
// "=====[  COMPUTERS  ]=====".
func groupHeader(group string, width int) string {
	title := group
	if optGroup.value() == "Category" {
		if t, ok := categoryTitles[group]; ok {
			title = t
		}
	}
	upper := strings.ToUpper(title)
	var core string
	var fill byte
	switch optHeaders.value() {
	case "Line":
		core, fill = " "+upper+" ", '-'
	case "Double":
		core, fill = " "+upper+" ", '='
	case "Brackets":
		core, fill = "[  "+upper+"  ]", '='
	case "Dots":
		core, fill = "  "+title+"  ", '.'
	default: // Minimal
		core, fill = ":: "+title+" ::", ' '
	}
	line := width - 2
	if line < len(core) {
		return core
	}
	left := (line - len(core)) / 2
	right := line - len(core) - left
	return " " + strings.TrimRight(strings.Repeat(string(fill), left)+core+strings.Repeat(string(fill), right), " ")
}

// systemListItems builds the systems list's lines: the labels, with a
// heading above each first-level group when headers are on. It returns
// the lines, which of them are headings, and for each line the index of
// its system in names (-1 for headings).
func systemListItems(names []string, width int) ([]string, map[int]bool, []int) {
	labels := systemLabels(names, width)
	useHeaders := optHeaders.value() != "Off" && optGroup.value() != "None"

	var items []string
	var index []int
	headers := map[int]bool{}
	last := "\x00"
	for i, name := range names {
		if useHeaders {
			if g := games.SystemGroup(name, optGroup.value()); g != last {
				headers[len(items)] = true
				items = append(items, groupHeader(g, width))
				index = append(index, -1)
				last = g
			}
		}
		items = append(items, labels[i])
		index = append(index, i)
	}
	return items, headers, index
}
