package gamesdb

import (
	"sort"
	"strings"

	"github.com/synrais/SAMenu/pkg/utils"
)

// Node is one folder in SAMenu tree. Subfolders are keyed by name
// in Children, and Files holds the games directly inside this folder.
type Node struct {
	Name     string
	Files    []FileInfo
	Children map[string]*Node
	// Pinned folders always come first, whatever the folder sorting (the
	// virtual [Games A-Z] folder).
	Pinned  bool
	discIdx []int // for a disc set: its discs' places in the parent folder
}

// AddAZFolder adds a pinned folder called name holding every game in this
// folder and all its subfolders, once each (same name and extension counts
// once), for browsing a set sorted into genres A-Z. Nothing is copied: it
// holds the same entries as the real folders.
func (n *Node) AddAZFolder(name string) {
	seen := map[string]bool{}
	var files []FileInfo
	var walk func(x *Node)
	walk = func(x *Node) {
		for _, f := range x.Files {
			k := strings.ToLower(f.Name + "." + f.Ext)
			if !seen[k] {
				seen[k] = true
				files = append(files, f)
			}
		}
		for _, c := range x.Children {
			if !c.Pinned {
				walk(c)
			}
		}
	}
	walk(n)
	az := newNode(name)
	az.Pinned = true
	az.Files = files
	n.Children[name] = az
}

// HasSubfolders reports whether this folder has real (not pinned) subfolders.
func (n *Node) HasSubfolders() bool {
	for _, c := range n.Children {
		if !c.Pinned {
			return true
		}
	}
	return false
}

// SystemID returns the system ID of the games in this folder.
func (n *Node) SystemID() string { return n.firstSystemId() }

// BuildTree turns database entries into a folder tree using each entry's
// MenuPath (e.g. "SNES/RPG/Chrono Trigger.sfc"); the first level is the
// systems. Every folder's games are sorted once here, by name (ignoring
// case) and then extension.
func BuildTree(files []FileInfo) *Node {
	root := newNode("")
	for _, f := range files {
		parts := strings.Split(f.MenuPath, "/")
		curr := root
		for _, part := range parts[:len(parts)-1] {
			child := curr.Children[part]
			if child == nil {
				child = newNode(part)
				curr.Children[part] = child
			}
			curr = child
		}
		curr.Files = append(curr.Files, f)
	}
	root.sortFiles()
	return root
}

// SortedFolders returns the names of this folder's subfolders, ignoring case.
func (n *Node) SortedFolders() []string {
	names := make([]string, 0, len(n.Children))
	for name := range n.Children {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		return utils.LessFold(names[i], names[j])
	})
	return names
}

func newNode(name string) *Node {
	return &Node{Name: name, Children: make(map[string]*Node)}
}

func (n *Node) sortFiles() {
	sort.SliceStable(n.Files, func(i, j int) bool {
		a, b := n.Files[i], n.Files[j]
		if a.Name != b.Name {
			return utils.LessFold(a.Name, b.Name)
		}
		return utils.LessFold(a.Ext, b.Ext)
	})
	for _, child := range n.Children {
		child.sortFiles()
	}
}

// RenameSystems names each top-level (system) folder from its games'
// system ID, using nameFor, so the menu follows the current system names
// even if the database was built before one was renamed.
func (n *Node) RenameSystems(nameFor func(id string) string) {
	for key, child := range n.Children {
		id := child.firstSystemId()
		if id == "" {
			continue
		}
		name := nameFor(id)
		if name == "" || name == key {
			continue
		}
		delete(n.Children, key)
		if existing, ok := n.Children[name]; ok { // merge, just in case
			existing.merge(child)
		} else {
			child.Name = name
			n.Children[name] = child
		}
	}
}

// firstSystemId returns the system ID of any game in this folder or below.
func (n *Node) firstSystemId() string {
	for _, f := range n.Files {
		if f.SystemId != "" {
			return f.SystemId
		}
	}
	for _, c := range n.Children {
		if id := c.firstSystemId(); id != "" {
			return id
		}
	}
	return ""
}

func (n *Node) merge(other *Node) {
	n.Files = append(n.Files, other.Files...)
	for k, c := range other.Children {
		if mine, ok := n.Children[k]; ok {
			mine.merge(c)
		} else {
			n.Children[k] = c
		}
	}
}
