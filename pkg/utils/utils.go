package utils

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"unicode"

	"golang.org/x/exp/constraints"
	"golang.org/x/text/unicode/norm"
)

// ListZip returns a slice of all filenames in a zip file.
func ListZip(path string) ([]string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var files []string
	for _, f := range r.File {
		files = append(files, f.Name)
	}

	return files, nil
}

func CopyFile(sourcePath, destPath string) error {
	inputFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer inputFile.Close()

	outputFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(outputFile, inputFile); err != nil {
		outputFile.Close()
		return err
	}
	// Sync and Close errors mean the copy may not have been written.
	if err := outputFile.Sync(); err != nil {
		outputFile.Close()
		return err
	}
	return outputFile.Close()
}

// Min returns the lowest value in a slice.
func Min[T constraints.Ordered](xs []T) T {
	if len(xs) == 0 {
		var zv T
		return zv
	}
	min := xs[0]
	for _, x := range xs {
		if x < min {
			min = x
		}
	}
	return min
}

// MapKeys returns a list of all keys in a map.
func MapKeys[K comparable, V any](m map[K]V) []K {
	keys := make([]K, len(m))
	i := 0
	for k := range m {
		keys[i] = k
		i++
	}
	return keys
}

func AlphaMapKeys[V any](m map[string]V) []string {
	keys := MapKeys(m)
	sort.Strings(keys)
	return keys
}

func RemoveFileExt(s string) string {
	return strings.TrimSuffix(s, filepath.Ext(s))
}

// ParseLine extracts (timestamp, path) from a gamelist entry.
// If no timestamp is present, ts = 0.
func ParseLine(line string) (float64, string) {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "<") {
		if idx := strings.Index(line, ">"); idx != -1 {
			tsStr := strings.TrimSpace(line[1:idx])
			path := strings.TrimSpace(line[idx+1:])
			if t, err := strconv.ParseFloat(tsStr, 64); err == nil {
				return t, path
			}
			// malformed timestamp: ignore, return as plain path
			return 0, path
		}
	}
	return 0, line
}

// --- Normalization helpers ---

// NormalizeTitle turns a game title into a comparison key: lowercase,
// Unicode-decomposed, letters and numbers only, single spaces. Unlike
// NormalizeEntry it doesn't treat anything as a file extension, so titles
// like "Dr. Mario" stay whole.
func NormalizeTitle(title string) string {
	name := norm.NFKD.String(strings.ToLower(title))

	// Walk runes: keep letters, numbers, spaces. Drop punctuation only.
	var b strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
		} else if unicode.IsSpace(r) {
			b.WriteRune(' ')
		}
	}

	// Collapse multiple spaces into one
	return strings.Join(strings.Fields(b.String()), " ")
}

// LessFold reports whether a sorts before b, ignoring case.
// Names that differ only in case fall back to exact order, so sorting is stable.
func LessFold(a, b string) bool {
	la, lb := strings.ToLower(a), strings.ToLower(b)
	if la != lb {
		return la < lb
	}
	return a < b
}
