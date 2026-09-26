package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	gc "github.com/rthornton128/goncurses"

	"github.com/synrais/SAMenu/pkg/config"
	"github.com/synrais/SAMenu/pkg/curses"
	"github.com/synrais/SAMenu/pkg/utils"
	"github.com/synrais/SAMenu/pkg/video"
)

// -------------------------
// Video Player
// -------------------------
//
// Options -> Video Player: browse /media/fat/video and play a video (or a
// whole folder), and the settings for videos in attract mode ([Video]).

// videoAttractChoices are the "In attract mode" steps: 0 = never.
var videoAttractChoices = []int{0, 3, 5, 10, 20}

func videoAttractText(n int) string {
	if n <= 0 {
		return "Off"
	}
	return fmt.Sprintf("Every %d games", n)
}

// videoFolderName shows a playlist setting ("" is the video folder itself).
func videoFolderName(p string) string {
	if p == "" {
		return "Video folder"
	}
	return p
}

// videoScreen is Options -> Video Player.
func videoScreen(stdscr *gc.Window, cfg *config.Config) {
	v := &cfg.Video
	selected := 0
	for {
		items := []string{
			"Play videos...",
			fmt.Sprintf("Play playlist (%s, %s)", videoFolderName(v.Playlist), strings.ToLower(v.Playback)),
			fmt.Sprintf("%-22s %s", "Playback:", v.Playback),
			fmt.Sprintf("%-22s %s", "Playlist:", videoFolderName(v.Playlist)),
			fmt.Sprintf("%-22s %s", "In attract mode:", videoAttractText(v.AttractEvery)),
			// Sync settings, mostly for sound drifting after seeking.
			fmt.Sprintf("%-32s %s", "Fast A/V resync (all files):", onOffText(v.AutoSync)),
			fmt.Sprintf("%-32s %s", "Timestamps (MP4, MKV):", onOffText(v.CorrectPts)),
			fmt.Sprintf("%-32s %s", "Accurate seek (MP3 audio):", onOffText(v.Mp3Seek)),
			fmt.Sprintf("%-32s %s", "Build index (AVI, no index):", onOffText(v.AviIndex)),
		}
		sel, ok := optionsList(stdscr, "Video Player", items, selected)
		if !ok {
			return
		}
		selected = sel
		switch sel {
		case 0:
			videoBrowser(stdscr, video.Folder)
			continue
		case 1:
			files := video.Videos(v.Playlist)
			if len(files) == 0 {
				message(stdscr, "No videos found in "+filepath.Join(video.Folder, v.Playlist))
				continue
			}
			playVideos(stdscr, files, v.Playback == "Random")
			continue
		case 2:
			if v.Playback == "Random" {
				v.Playback = "In order"
			} else {
				v.Playback = "Random"
			}
		case 3:
			lists := video.Playlists()
			next := 0
			for i, p := range lists {
				if p == v.Playlist {
					next = (i + 1) % len(lists)
				}
			}
			v.Playlist = lists[next]
		case 4:
			next := 0
			for i, n := range videoAttractChoices {
				if n == v.AttractEvery {
					next = (i + 1) % len(videoAttractChoices)
				}
			}
			v.AttractEvery = videoAttractChoices[next]
		case 5:
			v.AutoSync = !v.AutoSync
		case 6:
			v.CorrectPts = !v.CorrectPts
		case 7:
			v.Mp3Seek = !v.Mp3Seek
		case 8:
			v.AviIndex = !v.AviIndex
		}
		if err := config.SaveValues(cfg.Path, "Video", [][2]string{
			{"Playback", v.Playback}, {"Playlist", v.Playlist}, {"AttractEvery", fmt.Sprint(v.AttractEvery)},
			{"AutoSync", boolText(v.AutoSync)}, {"CorrectPts", boolText(v.CorrectPts)},
			{"Mp3Seek", boolText(v.Mp3Seek)}, {"AviIndex", boolText(v.AviIndex)},
		}); err != nil {
			message(stdscr, fmt.Sprintf("Couldn't save: %v", err))
		}
	}
}

// videoBrowser lists a folder: "[Play all]" (when it has videos), its
// subfolders, then its videos. Picking a video plays just that one.
func videoBrowser(stdscr *gc.Window, dir string) {
	if _, err := os.Stat(dir); err != nil {
		message(stdscr, "No video folder yet: put videos in "+video.Folder+"\n(folders inside it are playlists).")
		return
	}
	selected := 0
	for {
		var folders []string
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if e.IsDir() {
				folders = append(folders, e.Name())
			}
		}
		sort.Slice(folders, func(i, j int) bool { return utils.LessFold(folders[i], folders[j]) })
		files := video.InFolder(dir)

		var items []string
		playAll := len(files) > 1
		if playAll {
			items = append(items, "[Play all]")
		}
		for _, f := range folders {
			items = append(items, f+"/")
		}
		for _, f := range files {
			items = append(items, filepath.Base(f))
		}
		if len(items) == 0 {
			message(stdscr, "No videos here.")
			return
		}

		title := "Videos"
		if rel, err := filepath.Rel(video.Folder, dir); err == nil && rel != "." {
			title = rel
		}
		first := 0
		if playAll {
			first = 1
		}
		stdscr.Clear()
		stdscr.Refresh()
		button, sel, err := curses.ListPicker(stdscr, curses.ListPickerOpts{
			Title:         title,
			Buttons:       []string{"PgUp", "PgDn", "", "Back"},
			ActionButton:  2,
			DefaultButton: 2,
			SnapToAction:  true,
			ShowTotal:     true,
			Width:         systemListWidth,
			Height:        listHeight,
			InitialIndex:  selected,
			DynamicActionLabel: func(i int) string {
				if i >= first && i < first+len(folders) {
					return "Open"
				}
				return "Play"
			},
		}, items)
		if err != nil || button != 2 || sel < 0 {
			return
		}
		selected = sel
		switch {
		case playAll && sel == 0:
			playVideos(stdscr, files, false)
		case sel < first+len(folders):
			videoBrowser(stdscr, filepath.Join(dir, folders[sel-first]))
		default:
			playVideos(stdscr, []string{files[sel-first-len(folders)]}, false)
		}
	}
}

// playVideos hands the screen to the video player, then brings the menu
// back at its own text size.
func playVideos(stdscr *gc.Window, files []string, shuffle bool) {
	video.Stop() // a video started over SSH
	gc.End()
	err := video.PlayOnScreen(files, shuffle)

	// The video player changed the screen size: set the menu's again.
	textSize.current = 0
	stdscr.Refresh()
	applyTextSizeLive(stdscr)
	if err != nil {
		message(stdscr, fmt.Sprintf("Couldn't play: %v", err))
	}
}
