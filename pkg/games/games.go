package games

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/synrais/SAMenu/pkg/utils"
)

// DisplayName is a system's name, or its ID if it isn't a known system.
func DisplayName(id string) string {
	if system, ok := Systems[id]; ok {
		return system.Name
	}
	return id
}

// GetSystem looks up an exact system definition by ID.
func GetSystem(id string) (*System, error) {
	if system, ok := Systems[id]; ok {
		return &system, nil
	} else {
		return nil, fmt.Errorf("unknown system: %s", id)
	}
}

// MatchSystemFile returns true if a given file's extension is valid for a system.
func MatchSystemFile(system System, path string) bool {
	// ignore dot files
	if strings.HasPrefix(filepath.Base(path), ".") {
		return false
	}

	for _, args := range system.Slots {
		for _, ext := range args.Exts {
			if strings.HasSuffix(strings.ToLower(path), ext) {
				return true
			}
		}
	}
	return false
}

func AllSystems() []System {
	var systems []System

	keys := utils.AlphaMapKeys(Systems)
	for _, k := range keys {
		systems = append(systems, Systems[k])
	}
	return systems
}

type resultsStack [][]string

func (r *resultsStack) new() {
	*r = append(*r, []string{})
}

func (r *resultsStack) pop() {
	if len(*r) == 0 {
		return
	}
	*r = (*r)[:len(*r)-1]
}

func (r *resultsStack) get() (*[]string, error) {
	if len(*r) == 0 {
		return nil, fmt.Errorf("nothing on stack")
	}
	return &(*r)[len(*r)-1], nil
}

// GetFiles searches for all valid games in a given path and returns a list of files.
func GetFiles(systemId string, path string) ([]string, error) {
	var allResults []string
	var stack resultsStack
	visited := make(map[string]struct{})

	system, err := GetSystem(systemId)
	if err != nil {
		return nil, err
	}

	var scanner func(path string, file fs.DirEntry, err error) error
	scanner = func(path string, file fs.DirEntry, walkErr error) error {
		// An entry that can't be read is skipped, not fatal.
		if walkErr != nil || file == nil {
			return nil
		}

		// avoid recursive symlinks
		if file.IsDir() {
			if _, ok := visited[path]; ok {
				return filepath.SkipDir
			} else {
				visited[path] = struct{}{}
			}
		}

		// handle symlinked directories
		if file.Type()&os.ModeSymlink != 0 {
			// A broken link (its target renamed or deleted, e.g. an old
			// _Organized shortcut to an MRA) is skipped, not fatal.
			realPath, err := filepath.EvalSymlinks(path)
			if err != nil {
				return nil
			}

			file, err := os.Stat(realPath)
			if err != nil {
				return nil
			}

			if file.IsDir() {
				stack.new()
				defer stack.pop()

				err = filepath.WalkDir(realPath, scanner)
				if err != nil {
					return err
				}

				results, err := stack.get()
				if err != nil {
					return err
				}

				for i := range *results {
					allResults = append(allResults, strings.Replace((*results)[i], realPath, path, 1))
				}
				return nil
			}
		}

		results, err := stack.get()
		if err != nil {
			return err
		}

		if strings.HasSuffix(strings.ToLower(path), ".zip") {
			// zip files
			zipFiles, err := utils.ListZip(path)
			if err != nil {
				// skip invalid zip files
				return nil
			}

			for i := range zipFiles {
				if MatchSystemFile(*system, zipFiles[i]) {
					abs := filepath.Join(path, zipFiles[i])
					*results = append(*results, abs)
				}
			}
		} else {
			// regular files
			if resultsEdge, err, ok := RunEdgeCase(system.Id, path); ok {
				if err == nil && len(resultsEdge) > 0 {
					*results = append(*results, resultsEdge...)
				}
			} else {
				if MatchSystemFile(*system, path) {
					*results = append(*results, path)
				}
			}
		}
		return nil
	}

	stack.new()
	defer stack.pop()

	root, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}

	var realPath string
	if root.Mode()&os.ModeSymlink == 0 {
		realPath = path
	} else {
		realPath, err = filepath.EvalSymlinks(path)
		if err != nil {
			return nil, nil // the system folder is a broken link: no games
		}
	}

	realRoot, err := os.Stat(realPath)
	if err != nil {
		return nil, nil
	}
	if !realRoot.IsDir() {
		return nil, fmt.Errorf("root is not a directory")
	}

	err = filepath.WalkDir(realPath, scanner)
	if err != nil {
		return nil, err
	}

	results, err := stack.get()
	if err != nil {
		return nil, err
	}
	allResults = append(allResults, *results...)

	if realPath != path {
		for i := range allResults {
			allResults[i] = strings.Replace(allResults[i], realPath, path, 1)
		}
	}

	return allResults, nil
}

type RbfInfo struct {
	Path      string
	Filename  string
	ShortName string
	MglName   string
}
