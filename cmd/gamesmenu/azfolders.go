package main

import (
	"strings"

	gc "github.com/rthornton128/goncurses"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/games"
	"github.com/synrais/SAM-GO/pkg/gamesdb"
)

// -------------------------
// Virtual A-Z Folders
// -------------------------
//
// Options -> Display & Sorting -> Virtual A-Z Folders. A ticked system gets
// a [Games A-Z] folder at the top, listing every game from all its folders
// once, A-Z. Handy for big sets sorted into genres (e.g. PlayStation) that
// are too big to keep a second A-Z copy of. Nothing is copied on disk.
// Saved as [Menu] VirtualAZFolders in SAM.ini.

const azFolderName = "[Games A-Z]"

// azSystems holds the lowercase IDs of the ticked systems.
var azSystems = map[string]bool{}

// currentTree is the menu's tree as last built, and treeDirty asks the
// systems screen to rebuild it (after the A-Z folders change).
var (
	currentTree *gamesdb.Node
	treeDirty   bool
)

// applyAZConfig loads the ticked systems (IDs or groups) from SAM.ini.
func applyAZConfig(m config.MenuConfig) {
	azSystems, _ = games.ResolveSystems(m.VirtualAZFolders)
}

// addAZFolders gives each ticked system with subfolders its A-Z folder.
// Systems whose games are all in one folder already don't need one.
func addAZFolders(tree *gamesdb.Node) {
	for _, system := range tree.Children {
		if azSystems[strings.ToLower(system.SystemID())] && system.HasSubfolders() {
			system.AddAZFolder(azFolderName)
		}
	}
}

// azFoldersScreen ticks which systems get an A-Z folder. Only systems with
// subfolders are listed, in the systems screen's order.
func azFoldersScreen(stdscr *gc.Window, cfg *config.Config, sysNames []string) {
	var ids []string
	for _, id := range systemIDs(sysNames) {
		for _, node := range currentTree.Children {
			if strings.EqualFold(node.SystemID(), id) && node.HasSubfolders() {
				ids = append(ids, id)
				break
			}
		}
	}
	if len(ids) == 0 {
		message(stdscr, "None of your systems have subfolders,\nso there's nothing to add an A-Z folder to.")
		return
	}

	on := map[string]bool{}
	for _, id := range ids {
		on[id] = azSystems[strings.ToLower(id)]
	}
	tickSystems(stdscr, "Virtual A-Z Folders", ids, on, func(on map[string]bool) {
		var ticked []string
		for _, id := range ids {
			if on[id] {
				ticked = append(ticked, id)
			}
		}
		cfg.Menu.VirtualAZFolders = ticked
		applyAZConfig(cfg.Menu)
		if err := config.SaveVirtualAZFolders(cfg); err != nil {
			message(stdscr, "Couldn't save: "+err.Error())
		}
		treeDirty = true
	})
}
