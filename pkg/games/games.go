package games

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/synrais/SAM-GO/pkg/config"
	"github.com/synrais/SAM-GO/pkg/utils"
)

// --- Global extension registry ---
var systemExts map[string]map[string]struct{}

// GetSystem looks up an exact system definition by ID.
func GetSystem(id string) (*System, error) {
	if system, ok := Systems[id]; ok {
		return &system, nil
	}
	return nil, fmt.Errorf("unknown system: %s", id)
}

func GetGroup(groupId string) (System, error) {
	var merged System
	if _, ok := CoreGroups[groupId]; !ok {
		return merged, fmt.Errorf("no system group found for %s", groupId)
	}
	if len(CoreGroups[groupId]) < 1 {
		return merged, fmt.Errorf("no systems in %s", groupId)
	} else if len(CoreGroups[groupId]) == 1 {
		return CoreGroups[groupId][0], nil
	}
	merged = CoreGroups[groupId][0]
	merged.Slots = make([]Slot, 0)
	for _, s := range CoreGroups[groupId] {
		merged.Slots = append(merged.Slots, s.Slots...)
	}
	return merged, nil
}

// LookupSystem case-insensitively looks up system ID definition including aliases.
func LookupSystem(id string) (*System, error) {
	if system, err := GetGroup(id); err == nil {
		return &system, nil
	}
	for k, v := range Systems {
		if strings.EqualFold(k, id) {
			return &v, nil
		}
		for _, alias := range v.Alias {
			if strings.EqualFold(alias, id) {
				return &v, nil
			}
		}
	}
	return nil, fmt.Errorf("unknown system: %s", id)
}

// MatchSystemFile returns true if a given file's extension is valid for a system.
// Priority: .mgl > everything else
func MatchSystemFile(system System, path string) bool {
	// ignore dot files
	if strings.HasPrefix(filepath.Base(path), ".") {
		return false
	}

	ext := strings.ToLower(filepath.Ext(path))

	// .mgl always allowed
	if ext == ".mgl" {
		return true
	}

	// check precomputed map
	if exts, ok := systemExts[system.Id]; ok {
		_, ok := exts[ext]
		return ok
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

func AllSystemsExcept(excluded []string) []System {
	var systems []System
	excludeMap := make(map[string]bool)
	for _, e := range excluded {
		excludeMap[strings.TrimSpace(e)] = true
	}
	keys := utils.AlphaMapKeys(Systems)
	for _, k := range keys {
		sys := Systems[k]
		if containsFold(excludeMap, sys.Id) {
			continue
		}
		skip := false
		for _, alias := range sys.Alias {
			if containsFold(excludeMap, alias) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		systems = append(systems, sys)
	}
	return systems
}

// helper: case-insensitive key lookup
func containsFold(m map[string]bool, key string) bool {
	for k := range m {
		if strings.EqualFold(k, key) {
			return true
		}
	}
	return false
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

// GetFiles searches for all valid games in a given path and return a list of files.
// Priority rule: if a .mgl exists for a game, ignore its sibling non-mgl files.
func GetFiles(systemId string, path string) ([]string, error) {
	var allResults []string
	var stack resultsStack
	visited := make(map[string]struct{})

	system, err := GetSystem(systemId)
	if err != nil {
		return nil, err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	var scanner func(path string, file fs.DirEntry, err error) error
	scanner = func(path string, file fs.DirEntry, _ error) error {
		// avoid recursive symlinks
		if file.IsDir() {
			if _, ok := visited[path]; ok {
				return filepath.SkipDir
			}
			visited[path] = struct{}{}
		}

		// handle symlinked directories
		if file.Type()&os.ModeSymlink != 0 {
			err = os.Chdir(filepath.Dir(path))
			if err != nil {
				return err
			}
			realPath, err := filepath.EvalSymlinks(path)
			if err != nil {
				return err
			}
			file, err := os.Stat(realPath)
			if err != nil {
				return err
			}
			if file.IsDir() {
				err = os.Chdir(path)
				if err != nil {
					return err
				}
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

		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".zip" {
			zipFiles, err := utils.ListZip(path)
			if err != nil {
				return nil
			}
			for i := range zipFiles {
				if MatchSystemFile(*system, zipFiles[i]) {
					abs := filepath.Join(path, zipFiles[i])
					*results = append(*results, abs)
				}
			}
		} else if MatchSystemFile(*system, path) {
			*results = append(*results, path)
		}
		return nil
	}

	stack.new()
	defer stack.pop()

	root, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	err = os.Chdir(filepath.Dir(path))
	if err != nil {
		return nil, err
	}

	var realPath string
	if root.Mode()&os.ModeSymlink == 0 {
		realPath = path
	} else {
		realPath, err = filepath.EvalSymlinks(path)
		if err != nil {
			return nil, err
		}
	}
	realRoot, err := os.Stat(realPath)
	if err != nil {
		return nil, err
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
	err = os.Chdir(cwd)
	if err != nil {
		return nil, err
	}

	// enforce .mgl priority: drop non-mgl siblings if a .mgl exists
	finalResults := filterMglPriority(allResults)

	return finalResults, nil
}

func filterMglPriority(files []string) []string {
	mglMap := make(map[string]string)
	for _, f := range files {
		if strings.HasSuffix(strings.ToLower(f), ".mgl") {
			base := strings.TrimSuffix(filepath.Base(f), filepath.Ext(f))
			mglMap[base] = f
		}
	}

	if len(mglMap) == 0 {
		return files
	}

	var results []string
	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f))
		if ext == ".mgl" {
			results = append(results, f)
			continue
		}
		base := strings.TrimSuffix(filepath.Base(f), ext)
		if _, ok := mglMap[base]; !ok {
			results = append(results, f)
		}
	}
	return results
}

func GetAllFiles(systemPaths map[string][]string, statusFn func(systemId string, path string)) ([][2]string, error) {
	var allFiles [][2]string
	for systemId, paths := range systemPaths {
		for i := range paths {
			statusFn(systemId, paths[i])
			files, err := GetFiles(systemId, paths[i])
			if err != nil {
				return nil, err
			}
			for i := range files {
				allFiles = append(allFiles, [2]string{systemId, files[i]})
			}
		}
	}
	return allFiles, nil
}

func FilterUniqueFilenames(files []string) []string {
	var filtered []string
	filenames := make(map[string]struct{})
	for i := range files {
		fn := filepath.Base(files[i])
		if _, ok := filenames[fn]; ok {
			continue
		}
		filenames[fn] = struct{}{}
		filtered = append(filtered, files[i])
	}
	return filtered
}

var zipRe = regexp.MustCompile(`^(.*\.zip)/(.+)$`)

func FileExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	zipMatch := zipRe.FindStringSubmatch(path)
	if zipMatch != nil {
		zipPath := zipMatch[1]
		file := zipMatch[2]
		zipFiles, err := utils.ListZip(zipPath)
		if err != nil {
			return false
		}
		for i := range zipFiles {
			if zipFiles[i] == file {
				return true
			}
		}
	}
	return false
}

type RbfInfo struct {
	Path      string
	Filename  string
	ShortName string
	MglName   string
}

func ParseRbf(path string) RbfInfo {
	info := RbfInfo{
		Path:     path,
		Filename: filepath.Base(path),
	}
	if strings.Contains(info.Filename, "_") {
		info.ShortName = info.Filename[0:strings.LastIndex(info.Filename, "_")]
	} else {
		info.ShortName = strings.TrimSuffix(info.Filename, filepath.Ext(info.Filename))
	}
	if strings.HasPrefix(path, config.SdFolder) {
		relDir := strings.TrimPrefix(filepath.Dir(path), config.SdFolder+"/")
		info.MglName = filepath.Join(relDir, info.ShortName)
	} else {
		info.MglName = path
	}
	return info
}

func shallowScanRbf() ([]RbfInfo, error) {
	results := make([]RbfInfo, 0)
	isRbf := func(file os.DirEntry) bool {
		return filepath.Ext(strings.ToLower(file.Name())) == ".rbf"
	}
	infoSymlink := func(path string) (RbfInfo, error) {
		info, err := os.Lstat(path)
		if err != nil {
			return RbfInfo{}, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			newPath, err := os.Readlink(path)
			if err != nil {
				return RbfInfo{}, err
			}
			return ParseRbf(newPath), nil
		}
		return ParseRbf(path), nil
	}
	files, err := os.ReadDir(config.SdFolder)
	if err != nil {
		return results, err
	}
	for _, file := range files {
		if file.IsDir() && strings.HasPrefix(file.Name(), "_") {
			subFiles, err := os.ReadDir(filepath.Join(config.SdFolder, file.Name()))
			if err != nil {
				continue
			}
			for _, subFile := range subFiles {
				if isRbf(subFile) {
					path := filepath.Join(config.SdFolder, file.Name(), subFile.Name())
					info, err := infoSymlink(path)
					if err != nil {
						continue
					}
					results = append(results, info)
				}
			}
		} else if isRbf(file) {
			path := filepath.Join(config.SdFolder, file.Name())
			info, err := infoSymlink(path)
			if err != nil {
				continue
			}
			results = append(results, info)
		}
	}
	return results, nil
}

func SystemsWithRbf() map[string]RbfInfo {
	results := make(map[string]RbfInfo)
	rbfFiles, err := shallowScanRbf()
	if err != nil {
		return results
	}
	for _, rbfFile := range rbfFiles {
		for _, system := range Systems {
			shortName := system.Rbf
			if strings.Contains(shortName, "/") {
				shortName = shortName[strings.LastIndex(shortName, "/")+1:]
			}
			if strings.EqualFold(rbfFile.ShortName, shortName) {
				results[system.Id] = rbfFile
			}
		}
	}
	return results
}

// --- Precompute extension maps ---
func init() {
	systemExts = make(map[string]map[string]struct{})
	for _, sys := range Systems {
		extMap := make(map[string]struct{})
		for _, slot := range sys.Slots {
			for _, ext := range slot.Exts {
				extMap[strings.ToLower(ext)] = struct{}{}
			}
		}
		// also add .mgl globally
		extMap[".mgl"] = struct{}{}
		systemExts[sys.Id] = extMap
	}
}

