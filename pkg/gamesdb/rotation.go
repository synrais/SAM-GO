package gamesdb

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Arcade orientation
//
// Arcade MRA files state their screen orientation in a <rotation> line:
// "horizontal", "horizontal (180)", "vertical (cw)", "vertical (ccw)",
// "vertical(ccw)" or just "vertical". It's read into the database as the
// games are indexed, for attract mode's Orientation setting.

var rotationTag = regexp.MustCompile(`(?i)<rotation>\s*([^<]*?)\s*</rotation>`)

// ReadRotation reads an MRA's orientation, normalised to "horizontal",
// "vertical cw", "vertical ccw" or "vertical", or "" if it doesn't say.
func ReadRotation(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	b, _ := io.ReadAll(io.LimitReader(f, 256*1024)) // MRAs are small
	m := rotationTag.FindSubmatch(b)
	if m == nil {
		return ""
	}
	return normaliseRotation(string(m[1]))
}

func normaliseRotation(v string) string {
	v = strings.ToLower(strings.Join(strings.Fields(v), ""))
	switch {
	case strings.HasPrefix(v, "horizontal"): // incl. "horizontal (180)"
		return "horizontal"
	case strings.HasPrefix(v, "vertical") && strings.Contains(v, "ccw"):
		return "vertical ccw"
	case strings.HasPrefix(v, "vertical") && strings.Contains(v, "cw"):
		return "vertical cw"
	case strings.HasPrefix(v, "vertical"):
		return "vertical"
	}
	return ""
}

// FillRotations gives MRAs with no <rotation> line the orientation the
// other versions of the same game agree on: those in the same folder
// (e.g. _alternatives/_Gang Wars), or with the same title before any
// brackets. If they don't agree, or there are none, it stays unknown.
func FillRotations(files []FileInfo) {
	byDir := map[string]map[string]bool{}
	byTitle := map[string]map[string]bool{}
	note := func(m map[string]map[string]bool, key, rot string) {
		if m[key] == nil {
			m[key] = map[string]bool{}
		}
		m[key][rot] = true
	}
	for _, f := range files {
		if f.Rotation != "" {
			note(byDir, filepath.Dir(f.Path), f.Rotation)
			note(byTitle, baseTitle(f.Name), f.Rotation)
		}
	}
	agreed := func(set map[string]bool) string {
		if len(set) != 1 {
			return ""
		}
		for r := range set {
			return r
		}
		return ""
	}
	for i := range files {
		f := &files[i]
		if f.Rotation != "" || !strings.EqualFold(f.Ext, "mra") {
			continue
		}
		if r := agreed(byDir[filepath.Dir(f.Path)]); r != "" {
			f.Rotation = r
		} else if r := agreed(byTitle[baseTitle(f.Name)]); r != "" {
			f.Rotation = r
		}
	}
}

// baseTitle is a game's name before any ( ) or [ ], lower case: "Gang Wars
// (Japan)" -> "gang wars".
func baseTitle(name string) string {
	if i := strings.IndexAny(name, "(["); i >= 0 {
		name = name[:i]
	}
	return strings.ToLower(strings.TrimSpace(name))
}

// RotationMatches reports whether a game suits an [Attract] Orientation
// choice: "Both", "Horizontal", "Vertical", "Vertical CW" or "Vertical
// CCW". Unknown orientations only suit "Both". A plain "vertical" suits
// both vertical directions.
func RotationMatches(rot, choice string) bool {
	switch strings.ToLower(choice) {
	case "horizontal":
		return rot == "horizontal"
	case "vertical":
		return strings.HasPrefix(rot, "vertical")
	case "vertical cw":
		return rot == "vertical cw" || rot == "vertical"
	case "vertical ccw":
		return rot == "vertical ccw" || rot == "vertical"
	}
	return true // Both
}
