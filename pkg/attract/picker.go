package attract

import (
	"math"
	"math/rand"
	"sort"
	"strings"

	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/games"
	"github.com/synrais/SAMenu/pkg/gamesdb"
	"github.com/synrais/SAMenu/pkg/utils"
)

// Game picker
//
// How attract mode (and SAMenu's [Pick Random Game], and -random)
// choose a game, from [Attract] in SAMenu.ini:
//
//	Selection   who gets picked (SelectionModes):
//	  Per game         every game equally likely: big libraries dominate
//	  Per system       a random system, then a game: equal airtime
//	  Balanced         systems weighted by the square root of their size
//	  By category      a random category, then a system (balanced), then a game
//	  By manufacturer  the same with manufacturers
//	  Round robin      each system in turn (A-Z), a random game from each
//	  Time machine     each system in turn by release date, oldest first
//	  In order         every game in turn, system by system, A-Z
//	NoRepeats   every game plays once before any repeats (a shuffled deck
//	            per system)
//	MixSystems  never the same system twice in a row
//	OneVersion  one version per title: "Tetris (USA)", "Tetris (Europe)"
//	            and "Tetris (Japan)" count as one game (USA first), and a
//	            multi-disc game as one (disc 1)
//
// [Weights] multiplies a system's chance: "SNES = 3", "Computer = 0.5"
// (system IDs or groups; 0 = never).

// SelectionModes are the [Attract] Selection choices, in menu order.
var SelectionModes = []string{
	"Balanced", "Per game", "Per system", "By category", "By manufacturer",
	"Round robin", "Time machine", "In order",
}

// orderedModes play systems or games in turn, not at random.
var orderedModes = map[string]bool{"round robin": true, "time machine": true, "in order": true}

// Picker picks games from a pool.
type Picker struct {
	mode                  string // lowercase Selection
	noRepeats, mixSystems bool
	systems               []*sysPool // A-Z, or by release date for Time machine
	lastSystem            *sysPool
	cursor                int // next system in turn (Round robin, Time machine)
	title                 int // next title of the current system (In order)
	removed               map[string]bool
}

type sysPool struct {
	id, category, maker string
	titles              [][]gamesdb.FileInfo // each title's versions, best first
	deck                []int                // NoRepeats: titles left this round
	weight              float64
}

// NewPicker organises files for picking with cfg's [Attract] settings. For a
// single pick (SAMenu) the in-turn modes don't apply, so they use
// Balanced.
func NewPicker(cfg *config.Config, files []gamesdb.FileInfo, singlePick bool) *Picker {
	a := cfg.Attract
	p := &Picker{
		mode:       strings.ToLower(strings.TrimSpace(a.Selection)),
		noRepeats:  a.NoRepeats,
		mixSystems: a.MixSystems,
		removed:    map[string]bool{},
	}
	if p.mode == "" || (singlePick && orderedModes[p.mode]) {
		p.mode = "balanced"
	}

	// Group the files into systems, and each system's files into titles.
	byID := map[string]*sysPool{}
	titleIndex := map[string]map[string]int{}
	for _, f := range files {
		id := strings.ToLower(f.SystemId)
		sp := byID[id]
		if sp == nil {
			sp = &sysPool{id: f.SystemId, category: "Other", maker: "Other"}
			if s, err := games.GetSystem(f.SystemId); err == nil {
				sp.category = s.Category
				if s.Manufacturer != "" {
					sp.maker = s.Manufacturer
				}
			}
			byID[id] = sp
			titleIndex[id] = map[string]int{}
			p.systems = append(p.systems, sp)
		}
		key := f.Path
		if a.OneVersion {
			key = titleKey(f.Name)
		}
		if i, ok := titleIndex[id][key]; ok {
			sp.titles[i] = append(sp.titles[i], f)
		} else {
			titleIndex[id][key] = len(sp.titles)
			sp.titles = append(sp.titles, []gamesdb.FileInfo{f})
		}
	}

	for _, sp := range p.systems {
		for _, versions := range sp.titles {
			if len(versions) > 1 {
				sortVersions(versions)
			}
		}
		if p.mode == "in order" { // the only mode that uses the title order
			sortTitles(sp.titles)
		}
		n := float64(len(sp.titles))
		switch p.mode {
		case "per game":
			sp.weight = n
		case "per system":
			sp.weight = 1
		default: // balanced, and systems within a group
			sp.weight = math.Sqrt(n)
		}
		sp.weight *= customWeight(cfg, sp.id, sp.category, sp.maker)
	}

	sort.SliceStable(p.systems, func(i, j int) bool {
		a, b := p.systems[i], p.systems[j]
		if p.mode == "time machine" {
			da, db := releaseDate(a.id), releaseDate(b.id)
			if da != db {
				if da == "" || db == "" {
					return db == "" // undated last
				}
				return da < db
			}
		}
		return utils.LessFold(games.DisplayName(a.id), games.DisplayName(b.id))
	})
	return p
}

// Remove takes a game out of the pool (blacklisted, or won't launch).
func (p *Picker) Remove(path string) { p.removed[path] = true }

// Next picks the next game, or reports false when nothing is left.
func (p *Picker) Next() (gamesdb.FileInfo, bool) {
	if p.mode == "in order" {
		return p.nextInOrder()
	}
	for tries := 0; tries < 2*len(p.systems)+10; tries++ {
		sp := p.pickSystem()
		if sp == nil {
			return gamesdb.FileInfo{}, false
		}
		if f, ok := p.pickFrom(sp); ok {
			p.lastSystem = sp
			return f, true
		}
		sp.weight = 0 // nothing left in it
	}
	return gamesdb.FileInfo{}, false
}

// pickSystem chooses the system the next game comes from.
func (p *Picker) pickSystem() *sysPool {
	var live []*sysPool
	for _, sp := range p.systems {
		if sp.weight > 0 && p.hasGames(sp) {
			live = append(live, sp)
		}
	}
	if len(live) == 0 {
		return nil
	}

	// In turn: the next system with games.
	if p.mode == "round robin" || p.mode == "time machine" {
		for range p.systems {
			sp := p.systems[p.cursor%len(p.systems)]
			p.cursor++
			if sp.weight > 0 && p.hasGames(sp) {
				return sp
			}
		}
		return nil
	}

	// Never the same system twice in a row, if there's a choice.
	if p.mixSystems && len(live) > 1 && p.lastSystem != nil {
		for i, sp := range live {
			if sp == p.lastSystem {
				live = append(live[:i:i], live[i+1:]...)
				break
			}
		}
	}

	// By group: a random group first, then a system in it.
	if p.mode == "by category" || p.mode == "by manufacturer" {
		groups := map[string][]*sysPool{}
		var names []string
		for _, sp := range live {
			g := sp.category
			if p.mode == "by manufacturer" {
				g = sp.maker
			}
			if groups[g] == nil {
				names = append(names, g)
			}
			groups[g] = append(groups[g], sp)
		}
		sort.Strings(names)
		live = groups[names[rand.Intn(len(names))]]
	}

	total := 0.0
	for _, sp := range live {
		total += sp.weight
	}
	r := rand.Float64() * total
	for _, sp := range live {
		if r -= sp.weight; r < 0 {
			return sp
		}
	}
	return live[len(live)-1]
}

// pickFrom picks a title from a system (from its shuffled deck with
// NoRepeats), and that title's best version still in the pool.
func (p *Picker) pickFrom(sp *sysPool) (gamesdb.FileInfo, bool) {
	for refills := 0; refills < 2; refills++ {
		if !p.noRepeats {
			var live []int
			for i := range sp.titles {
				if _, ok := p.bestVersion(sp.titles[i]); ok {
					live = append(live, i)
				}
			}
			if len(live) == 0 {
				return gamesdb.FileInfo{}, false
			}
			f, _ := p.bestVersion(sp.titles[live[rand.Intn(len(live))]])
			return f, true
		}
		for len(sp.deck) > 0 {
			i := sp.deck[len(sp.deck)-1]
			sp.deck = sp.deck[:len(sp.deck)-1]
			if f, ok := p.bestVersion(sp.titles[i]); ok {
				return f, true
			}
		}
		// Deck used up: a new round, freshly shuffled.
		sp.deck = rand.Perm(len(sp.titles))
	}
	return gamesdb.FileInfo{}, false
}

// nextInOrder is In order: every title of every system in turn, A-Z.
func (p *Picker) nextInOrder() (gamesdb.FileInfo, bool) {
	total := 0
	for _, sp := range p.systems {
		total += len(sp.titles)
	}
	for step := 0; step < total+len(p.systems)+1; step++ {
		sp := p.systems[p.cursor%len(p.systems)]
		if p.title >= len(sp.titles) {
			p.cursor++
			p.title = 0
			continue
		}
		t := sp.titles[p.title]
		p.title++
		if sp.weight == 0 {
			continue
		}
		if f, ok := p.bestVersion(t); ok {
			return f, true
		}
	}
	return gamesdb.FileInfo{}, false
}

func (p *Picker) bestVersion(versions []gamesdb.FileInfo) (gamesdb.FileInfo, bool) {
	for _, v := range versions {
		if !p.removed[v.Path] {
			return v, true
		}
	}
	return gamesdb.FileInfo{}, false
}

func (p *Picker) hasGames(sp *sysPool) bool {
	for _, t := range sp.titles {
		if _, ok := p.bestVersion(t); ok {
			return true
		}
	}
	return false
}

// ----- titles and versions -----

// titleKey is a game's title without its tags: "Tetris (USA) (Rev 1)" and
// "Tetris [!]" are both "tetris".
func titleKey(name string) string {
	if i := strings.IndexAny(name, "(["); i > 0 {
		name = name[:i]
	}
	return strings.ToLower(strings.TrimSpace(name))
}

// regionOrder: earlier is preferred.
var regionOrder = []string{"usa", "world", "europe", "uk", "australia", "japan"}

// badTags make a version less wanted than a clean release.
var badTags = []string{"beta", "proto", "demo", "sample", "hack", "pirate", "unl", "[b"}

// versionScore ranks versions of a title: lower is better. Disc 1 first,
// then clean releases, then by region (USA, World, Europe, ...).
func versionScore(name string) int {
	l := strings.ToLower(name)
	score := 0
	if i := strings.Index(l, "disc "); i >= 0 && i+5 < len(l) && l[i+5] != '1' {
		score += 10000 // a later disc
	}
	for _, t := range badTags {
		if strings.Contains(l, t) {
			score += 1000
		}
	}
	region := len(regionOrder)
	for i, r := range regionOrder {
		if strings.Contains(l, r) {
			region = i
			break
		}
	}
	return score + region
}

// ----- systems -----

// customWeight multiplies all [Weights] entries that match a system: its ID,
// category and manufacturer. No entries = 1.
func customWeight(cfg *config.Config, id, category, maker string) float64 {
	w := 1.0
	for _, k := range []string{id, category, maker} {
		if v, ok := cfg.Weights[strings.ToLower(k)]; ok {
			w *= v
		}
	}
	return w
}

func releaseDate(id string) string {
	if s, err := games.GetSystem(id); err == nil {
		return s.ReleaseDate
	}
	return ""
}

// sortVersions puts a title's best version first (versionScore), working
// out each version's score once.
func sortVersions(versions []gamesdb.FileInfo) {
	scores := make([]int, len(versions))
	idx := make([]int, len(versions))
	for i, v := range versions {
		scores[i], idx[i] = versionScore(v.Name), i
	}
	sort.SliceStable(idx, func(a, b int) bool { return scores[idx[a]] < scores[idx[b]] })
	sorted := make([]gamesdb.FileInfo, len(versions))
	for i, j := range idx {
		sorted[i] = versions[j]
	}
	copy(versions, sorted)
}

// sortTitles sorts titles A-Z, working out each title's key once.
func sortTitles(titles [][]gamesdb.FileInfo) {
	keys := make([]string, len(titles))
	idx := make([]int, len(titles))
	for i, t := range titles {
		keys[i], idx[i] = titleKey(t[0].Name), i
	}
	sort.SliceStable(idx, func(a, b int) bool { return utils.LessFold(keys[idx[a]], keys[idx[b]]) })
	sorted := make([][]gamesdb.FileInfo, len(titles))
	for i, j := range idx {
		sorted[i] = titles[j]
	}
	copy(titles, sorted)
}
