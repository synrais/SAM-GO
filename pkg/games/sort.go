package games

import (
	"sort"
	"strings"

	"github.com/synrais/SAM-GO/pkg/utils"
)

// OtherManufacturer groups systems that have no manufacturer set.
const OtherManufacturer = "Other"

// arcadeId is the Arcade Cores system, which can be pinned to the top.
const arcadeId = "Arcade"

// DefaultCategoryOrder is used for Custom group order when none is set.
var DefaultCategoryOrder = []string{CategoryArcade, CategoryConsole, CategoryHandheld, CategoryComputer, CategoryOther}

// SortOptions controls how the systems list is ordered: an optional first
// grouping, an optional second grouping inside it, and the order of the
// systems inside the groups. The zero value is not the default; use
// DefaultSortOptions.
type SortOptions struct {
	Group         string   // first level: "Manufacturer", "Category" or "None"
	GroupOrder    string   // "A-Z", "Oldest first" or "Custom" (Category only)
	CategoryOrder []string // for Custom: categories in order, the rest after
	Subgroup      string   // second level: "Manufacturer", "Category" or "None"
	SubgroupOrder string   // "A-Z" or "Oldest first"
	Within        string   // systems: "Release date" or "A-Z"
	ArcadeFirst   bool     // Arcade Cores always at the top
}

// DefaultSortOptions is how the list has always been sorted: by
// manufacturer A-Z, then release date.
func DefaultSortOptions() SortOptions {
	return SortOptions{Group: "Manufacturer", GroupOrder: "A-Z", CategoryOrder: DefaultCategoryOrder,
		Subgroup: "None", SubgroupOrder: "A-Z", Within: "Release date", ArcadeFirst: true}
}

// SortSystemNames sorts system display names with the default options.
func SortSystemNames(names []string) { SortSystemNamesWith(names, DefaultSortOptions()) }

// SystemGroup is the group a system (by display name) falls in for a
// grouping: "Category", "Manufacturer" or "None" ("").
func SystemGroup(name, by string) string {
	s, ok := systemsByName()[name]
	if !ok {
		s = System{Name: name}
	}
	return groupKey(s, by)
}

// groupKey is a system's group for a grouping ("" for None).
func groupKey(s System, by string) string {
	switch by {
	case "Category":
		if s.Category == "" {
			return CategoryOther
		}
		return s.Category
	case "Manufacturer":
		return manufacturerOf(s)
	}
	return ""
}

// SortSystemNamesWith sorts system display names for menus, in place, e.g.
// by category (in a custom order), then manufacturer A-Z, then release
// date:
//
//	Console:  [Atari] 2600, [Atari] 7800, [Nintendo] NES, [Nintendo] SNES
//	Handheld: [Atari] Lynx, [Nintendo] Gameboy
//
// A group called "Other" (no manufacturer or category) goes last, unless a
// custom category order places it. Names that don't match a known system
// are treated as "Other".
func SortSystemNamesWith(names []string, o SortOptions) {
	byName := systemsByName()
	lookup := func(name string) System {
		if s, ok := byName[name]; ok {
			return s
		}
		return System{Name: name}
	}
	if o.Subgroup == o.Group {
		o.Subgroup = "None"
	}
	pinned := func(s System) bool { return o.ArcadeFirst && s.Id == arcadeId }

	// Each group's (and each group-within-group's) earliest release, for
	// "Oldest first".
	earliest := map[string]System{}
	note := func(key string, s System) {
		if e, ok := earliest[key]; !ok || releasedBefore(s, e) {
			earliest[key] = s
		}
	}
	for _, n := range names {
		s := lookup(n)
		if pinned(s) {
			continue
		}
		g := strings.ToLower(groupKey(s, o.Group))
		note("1|"+g, s)
		note("2|"+g+"|"+strings.ToLower(groupKey(s, o.Subgroup)), s)
	}

	rank := func(group string) int {
		for i, c := range o.CategoryOrder {
			if strings.EqualFold(strings.TrimSpace(c), group) {
				return i
			}
		}
		return len(o.CategoryOrder)
	}

	// compare orders two different groups at one level: <0, 0 or >0.
	compare := func(ga, gb, order string, custom bool, ea, eb System) int {
		if custom {
			if ra, rb := rank(ga), rank(gb); ra != rb {
				return ra - rb
			}
		} else {
			if ga == OtherManufacturer {
				return 1
			}
			if gb == OtherManufacturer {
				return -1
			}
			if order == "Oldest first" && ea.ReleaseDate != eb.ReleaseDate {
				if releasedBefore(ea, eb) {
					return -1
				}
				return 1
			}
		}
		if utils.LessFold(ga, gb) {
			return -1
		}
		return 1
	}

	sort.SliceStable(names, func(i, j int) bool {
		a, b := lookup(names[i]), lookup(names[j])
		if pa, pb := pinned(a), pinned(b); pa != pb {
			return pa
		}

		ga, gb := groupKey(a, o.Group), groupKey(b, o.Group)
		la, lb := strings.ToLower(ga), strings.ToLower(gb)
		if la != lb {
			custom := o.GroupOrder == "Custom" && o.Group == "Category"
			return compare(ga, gb, o.GroupOrder, custom, earliest["1|"+la], earliest["1|"+lb]) < 0
		}

		sa, sb := groupKey(a, o.Subgroup), groupKey(b, o.Subgroup)
		if !strings.EqualFold(sa, sb) {
			return compare(sa, sb, o.SubgroupOrder, false,
				earliest["2|"+la+"|"+strings.ToLower(sa)], earliest["2|"+lb+"|"+strings.ToLower(sb)]) < 0
		}

		if o.Within == "A-Z" || o.Within == "Alphabetical" {
			return utils.LessFold(a.Name, b.Name)
		}
		return releasedBefore(a, b)
	})
}

// SystemLabel returns a system's menu label, e.g. "[Philips] CD-i".
// A name that starts with its manufacturer drops the repeat, so
// "Atari 2600" shows as "[Atari] 2600".
func SystemLabel(name string) string {
	m, short := SystemLabelParts(name)
	return "[" + m + "] " + short
}

// SystemLabelParts returns a system's manufacturer ("Other" if unset) and
// its name with a repeated leading manufacturer removed, for building menu
// labels in any style.
func SystemLabelParts(name string) (manufacturer, short string) {
	s, ok := systemsByName()[name]
	if !ok {
		s = System{Name: name}
	}
	m := manufacturerOf(s)
	return m, trimManufacturer(name, m)
}

// trimManufacturer removes a leading "<manufacturer> " from name, ignoring
// case. The name is kept whole if nothing would be left.
func trimManufacturer(name, manufacturer string) string {
	prefix := manufacturer + " "
	if len(name) > len(prefix) && strings.EqualFold(name[:len(prefix)], prefix) {
		if rest := strings.TrimSpace(name[len(prefix):]); rest != "" {
			return rest
		}
	}
	return name
}

func systemsByName() map[string]System {
	byName := make(map[string]System, len(Systems))
	for _, s := range Systems {
		byName[s.Name] = s
	}
	return byName
}

func manufacturerOf(s System) string {
	if s.Manufacturer == "" {
		return OtherManufacturer
	}
	return s.Manufacturer
}

// releasedBefore orders two systems by release date. Systems without a date
// go last. A year-only placeholder date ("YYYY-01-01") is only compared by
// year, and systems that can't be told apart fall back to alphabetical.
func releasedBefore(a, b System) bool {
	da, db := a.ReleaseDate, b.ReleaseDate
	switch {
	case da == "" && db == "":
		return utils.LessFold(a.Name, b.Name)
	case da == "":
		return false
	case db == "":
		return true
	}

	if ya, yb := releaseYear(da), releaseYear(db); ya != yb {
		return ya < yb
	}
	if !isYearOnly(da) && !isYearOnly(db) && da != db {
		return da < db
	}
	return utils.LessFold(a.Name, b.Name)
}

func releaseYear(date string) string {
	if len(date) >= 4 {
		return date[:4]
	}
	return date
}

// isYearOnly reports whether a date is a "sometime that year" placeholder.
func isYearOnly(date string) bool {
	return strings.HasSuffix(date, "-01-01")
}

// ResolveSystems turns a list of system IDs and group names (as written in
// SAM.ini, e.g. "Console, Nintendo, AmigaVision") into the set of system IDs
// they cover. Each name is matched, ignoring case, against:
//
//   - a system ID          (e.g. SNES, AmigaVision)
//   - a category           (Console, Computer, Handheld, Arcade, Other)
//   - a manufacturer       (e.g. Nintendo, Sega, Commodore)
//
// A name can match more than one of these; all matches are added. The
// returned IDs are lowercase. Names that match nothing are returned in
// unknown so they can be reported.
func ResolveSystems(names []string) (ids map[string]bool, unknown []string) {
	ids = make(map[string]bool)
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		found := false
		for _, s := range Systems {
			if strings.EqualFold(s.Id, name) ||
				strings.EqualFold(s.Category, name) ||
				(s.Manufacturer != "" && strings.EqualFold(s.Manufacturer, name)) {
				ids[strings.ToLower(s.Id)] = true
				found = true
			}
		}
		if !found {
			unknown = append(unknown, name)
		}
	}
	return ids, unknown
}
