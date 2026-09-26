package gamesdb

import (
	"regexp"
	"strings"
)

// Name tags
//
// Tags in game file names, as No-Intro, Redump, TOSEC and GoodTools write
// them: "Tetris (USA) (Beta)", "Sonic [h1]", "FF7 (Disc 2)". SAMenu
// can hide games with chosen tags ([Menu] HideTags) and attract mode can
// skip them ([Attract] SkipTags).
//
// Tags only match inside ( ) or [ ] and as whole words, ignoring case, so
// Demo never catches "Demolition Man" and Hack never catches "Hacker".
// Translation also catches the unbracketed "T+Eng" style.

// Tag is one tick-box choice.
type Tag struct {
	Name string
	// word matches a whole bracket's text or words in it; code matches a
	// whole short bracket like [h1] or [b]; loose matches the whole name.
	word, code, loose *regexp.Regexp
	// hints: the (lowercase) name must contain one of these for the tag to
	// be worth checking properly. A quick test that rules out almost every
	// name before any pattern matching.
	hints []string
}

func tagRe(s string) *regexp.Regexp { return regexp.MustCompile(`(?i)` + s) }

// Tags are the choices, in menu order.
var Tags = []Tag{
	{Name: "Beta", word: tagRe(`\bbeta\b`), hints: []string{"beta"}},
	{Name: "Proto", word: tagRe(`\bproto(type)?\b`), hints: []string{"proto"}},
	{Name: "Demo", word: tagRe(`\b(demo|sample|kiosk|promo|preview)\b`), hints: []string{"demo", "sample", "kiosk", "promo", "preview"}},
	{Name: "Program", word: tagRe(`\b(program|test program|bios)\b`), hints: []string{"program", "bios"}},
	{Name: "Hack", word: tagRe(`\bhack\b`), code: tagRe(`^h\d*[a-z]{0,2}$`), hints: []string{"hack", "[h", "(h"}},
	{Name: "Translation", code: tagRe(`^t[+-]`), loose: tagRe(`(^|[\s(\[])t[+-][a-z]{2,3}\b`), hints: []string{"t+", "t-"}},
	{Name: "Unlicensed", word: tagRe(`\b(unl|unlicensed|pirate|bootleg)\b`), hints: []string{"unl", "pirate", "bootleg"}},
	{Name: "Homebrew", word: tagRe(`\b(homebrew|aftermarket)\b`), hints: []string{"homebrew", "aftermarket"}},
	{Name: "Cheat / Trainer / Fixed", code: tagRe(`^[ctf]\d*$`), hints: []string{"[c", "[t", "[f", "(c", "(t", "(f"}},
	{Name: "Bad dump", code: tagRe(`^[bo]\d*$`), hints: []string{"[b", "[o", "(b", "(o"}},
	{Name: "Alternate", code: tagRe(`^a\d*$`), hints: []string{"[a", "(a"}},
	{Name: "Disc 2+", word: tagRe(`\b(disc|disk|cd|side|tape)\s*([2-9]|[1-9]\d|b)\b`), hints: []string{"disc", "disk", "cd", "side", "tape"}},
	{Name: "Re-releases", word: tagRe(`\b(virtual console|switch online|collection|mini|evercade|steam)\b`), hints: []string{"virtual console", "switch online", "collection", "mini", "evercade", "steam"}},
}

// TagNames lists the tag choices, in menu order.
func TagNames() []string {
	names := make([]string, len(Tags))
	for i, t := range Tags {
		names[i] = t.Name
	}
	return names
}

// TagFilter tests game names against some tags. The zero value (no tags)
// matches nothing.
type TagFilter struct{ tags []Tag }

// NewTagFilter makes a filter for tag names (as saved in SAMenu.ini). Unknown
// names are ignored.
func NewTagFilter(names []string) TagFilter {
	var f TagFilter
	for _, n := range names {
		for _, t := range Tags {
			if strings.EqualFold(strings.TrimSpace(n), t.Name) {
				f.tags = append(f.tags, t)
			}
		}
	}
	return f
}

// Names lists the filter's tag names, in menu order.
func (f TagFilter) Names() []string {
	var names []string
	for _, t := range Tags {
		for _, ft := range f.tags {
			if ft.Name == t.Name {
				names = append(names, t.Name)
				break
			}
		}
	}
	return names
}

// Empty reports whether the filter has no tags.
func (f TagFilter) Empty() bool { return len(f.tags) == 0 }

// Matches reports whether a game name has any of the filter's tags.
func (f TagFilter) Matches(name string) bool {
	if len(f.tags) == 0 {
		return false
	}
	lower := strings.ToLower(name)
	var brackets []string
	found := false // brackets found yet (only looked for when a hint matches)
	for _, t := range f.tags {
		if !hinted(lower, t.hints) {
			continue // can't match: skip the pattern matching
		}
		if !found {
			brackets, found = bracketTexts(name), true
		}
		if t.loose != nil && t.loose.MatchString(name) {
			return true
		}
		for _, inside := range brackets {
			if t.word != nil && t.word.MatchString(inside) {
				return true
			}
			if t.code != nil && t.code.MatchString(inside) {
				return true
			}
		}
	}
	return false
}

// bracketTexts returns what's inside each ( ) or [ ] in a name, trimmed.
func bracketTexts(name string) []string {
	var out []string
	start := -1
	for i := 0; i < len(name); i++ {
		switch name[i] {
		case '(', '[':
			start = i + 1
		case ')', ']':
			if start >= 0 {
				out = append(out, strings.TrimSpace(name[start:i]))
				start = -1
			}
		}
	}
	return out
}

func hinted(lower string, hints []string) bool {
	if len(hints) == 0 {
		return true
	}
	for _, h := range hints {
		if strings.Contains(lower, h) {
			return true
		}
	}
	return false
}

// Without returns files minus the ones whose names have the filter's tags.
func (f TagFilter) Without(files []FileInfo) []FileInfo {
	if f.Empty() {
		return files
	}
	out := make([]FileInfo, 0, len(files))
	for _, file := range files {
		if !f.Matches(file.Name) {
			out = append(out, file)
		}
	}
	return out
}
