package gamesdb

import (
	"regexp"
	"sort"
	"strings"

	"github.com/synrais/SAMenu/pkg/utils"
)

// Game list ordering
//
// How folders and games are ordered in SAMenu and search results
// ([Menu] FolderPosition, GameOrder and IgnoreThe in SAMenu.ini).

// GameOrder decides how two names compare.
type GameOrder struct {
	Natural   bool // numbers by value: "Game 2" before "Game 10"
	IgnoreThe bool // "The Legend of Zelda" sorts under L
	// GroupDiscs puts multi-disc games ("(Disc 1)", "(Disc 2)"...) in a
	// folder of their own, listed in with the games.
	GroupDiscs bool
}

// Less reports whether name a sorts before name b.
func (o GameOrder) Less(a, b string) bool {
	ka, kb := a, b
	if o.IgnoreThe {
		ka, kb = stripThe(ka), stripThe(kb)
	}
	if o.Natural {
		if c := naturalCompare(ka, kb); c != 0 {
			return c < 0
		}
	} else if !strings.EqualFold(ka, kb) {
		return utils.LessFold(ka, kb)
	}
	return utils.LessFold(a, b) // same key: keep a stable, sensible order
}

// stripThe drops a leading "The " (any case).
func stripThe(s string) string {
	if len(s) > 4 && strings.EqualFold(s[:4], "the ") {
		return strings.TrimSpace(s[4:])
	}
	return s
}

// naturalCompare compares ignoring case, with runs of digits compared by
// their value. It returns -1, 0 or 1.
func naturalCompare(a, b string) int {
	a, b = strings.ToLower(a), strings.ToLower(b)
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		ca, cb := a[i], b[j]
		if isDigit(ca) && isDigit(cb) {
			si, sj := i, j
			for i < len(a) && isDigit(a[i]) {
				i++
			}
			for j < len(b) && isDigit(b[j]) {
				j++
			}
			na := strings.TrimLeft(a[si:i], "0")
			nb := strings.TrimLeft(b[sj:j], "0")
			if len(na) != len(nb) {
				if len(na) < len(nb) {
					return -1
				}
				return 1
			}
			if na != nb {
				if na < nb {
					return -1
				}
				return 1
			}
			continue
		}
		if ca != cb {
			if ca < cb {
				return -1
			}
			return 1
		}
		i++
		j++
	}
	switch {
	case len(a)-i < len(b)-j:
		return -1
	case len(a)-i > len(b)-j:
		return 1
	}
	return 0
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// Entry is one line of a folder listing: a subfolder or a game.
type Entry struct {
	Folder string    // set for a subfolder
	File   *FileInfo // set for a game
	Node   *Node     // set for a disc set: a folder made up for its discs
}

// Entries lists a folder's subfolders and games in the given order.
// folders is "First", "Last" or "Mixed" (in with the games, by name).
func (n *Node) Entries(o GameOrder, folders string) []Entry {
	var pinned, dirs, files []Entry
	for name, child := range n.Children {
		if child.Pinned {
			pinned = append(pinned, Entry{Folder: name})
		} else {
			dirs = append(dirs, Entry{Folder: name})
		}
	}
	grouped := map[int]bool{}
	if o.GroupDiscs {
		// Disc set folders go in with the games, whatever "folders" says.
		for _, set := range n.discSets() {
			files = append(files, Entry{Folder: set.Name, Node: set})
			for _, i := range set.discIdx {
				grouped[i] = true
			}
		}
	}
	for i := range n.Files {
		if !grouped[i] {
			files = append(files, Entry{File: &n.Files[i]})
		}
	}
	key := func(e Entry) string {
		if e.File != nil {
			return e.File.Name
		}
		return e.Folder
	}
	less := func(list []Entry) func(i, j int) bool {
		return func(i, j int) bool {
			a, b := list[i], list[j]
			ka, kb := key(a), key(b)
			if ka == kb && a.File != nil && b.File != nil {
				return utils.LessFold(a.File.Ext, b.File.Ext)
			}
			return o.Less(ka, kb)
		}
	}

	// Pinned folders always go first.
	sort.SliceStable(pinned, less(pinned))
	switch folders {
	case "Mixed":
		all := append(dirs, files...)
		sort.SliceStable(all, less(all))
		return append(pinned, all...)
	case "Last":
		sort.SliceStable(dirs, less(dirs))
		sort.SliceStable(files, less(files))
		return append(pinned, append(files, dirs...)...)
	default:
		sort.SliceStable(dirs, less(dirs))
		sort.SliceStable(files, less(files))
		return append(pinned, append(dirs, files...)...)
	}
}

// SortResultsBySystem orders search results the way the search screen
// always shows them: systems A-Z, then titles A-Z, then extension, all
// ignoring case and the menu's sort settings.
func SortResultsBySystem(results []SearchResult, systemName func(id string) string) {
	sort.SliceStable(results, func(i, j int) bool {
		a, b := results[i], results[j]
		if sa, sb := systemName(a.SystemId), systemName(b.SystemId); !strings.EqualFold(sa, sb) {
			return utils.LessFold(sa, sb)
		}
		if !strings.EqualFold(a.Name, b.Name) {
			return utils.LessFold(a.Name, b.Name)
		}
		return utils.LessFold(a.Ext, b.Ext)
	})
}

// discTag matches a disc number in a game's name: "(Disc 1)", "[Disc 2 of
// 3]", "(Disk A)", "(CD1)".
var discTag = regexp.MustCompile(`(?i)\s*[\(\[]\s*(?:disc|disk|cd)\s*(?:[0-9]+|[a-z])(?:\s*of\s*[0-9]+)?\s*[\)\]]`)

// discBase returns a game's name without its disc tag, and whether it had
// one: "Final Fantasy VII (USA) (Disc 1)" -> "Final Fantasy VII (USA)".
func discBase(name string) (string, bool) {
	loc := discTag.FindStringIndex(name)
	if loc == nil {
		return name, false
	}
	base := strings.Join(strings.Fields(name[:loc[0]]+" "+name[loc[1]:]), " ")
	return base, base != ""
}

// discSets finds the multi-disc games in this folder: two or more discs
// of the same game. A folder that already holds just one game's discs is
// left as it is, as is a set whose name matches a real subfolder.
func (n *Node) discSets() []*Node {
	byBase := map[string][]int{}
	var order []string
	for i, f := range n.Files {
		if base, ok := discBase(f.Name); ok {
			if _, seen := byBase[base]; !seen {
				order = append(order, base)
			}
			byBase[base] = append(byBase[base], i)
		}
	}
	var sets []*Node
	for _, base := range order {
		idx := byBase[base]
		if len(idx) < 2 || n.Children[base] != nil {
			continue
		}
		if len(idx) == len(n.Files) && len(n.Children) == 0 {
			continue // already grouped: this folder is the disc set
		}
		set := &Node{Name: base, Children: map[string]*Node{}, discIdx: idx}
		for _, i := range idx {
			set.Files = append(set.Files, n.Files[i])
		}
		sets = append(sets, set)
	}
	return sets
}
