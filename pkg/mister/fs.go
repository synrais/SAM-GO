package mister

import (
	"encoding/xml"
	"os"

	"github.com/synrais/SAMenu/pkg/config"
)

func GetActiveCoreName() (string, error) {
	data, err := os.ReadFile(config.CoreNameFile)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func ActiveGameEnabled() bool {
	_, err := os.Stat(config.ActiveGameFile)
	return err == nil
}

func SetActiveGame(path string) error {
	file, err := os.Create(config.ActiveGameFile)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(path)
	if err != nil {
		return err
	}

	return nil
}

type RecentEntry struct {
	Directory string
	Name      string
	Label     string
}

type MGLFile struct {
	XMLName xml.Name `xml:"file"`
	Delay   int      `xml:"delay,attr"`
	Type    string   `xml:"type,attr"`
	Index   int      `xml:"index,attr"`
	Path    string   `xml:"path,attr"`
}

type MGL struct {
	XMLName xml.Name `xml:"mistergamedescription"`
	Rbf     string   `xml:"rbf"`
	SetName string   `xml:"setname"`
	File    MGLFile  `xml:"file"`
}

type MenuConfig struct {
	BackgroundMode int
}

const (
	BackgroundModeNone      = 0
	BackgroundModeWallpaper = 2
	BackgroundModeHBars1    = 4
	BackgroundModeHBars2    = 6
	BackgroundModeVBars1    = 8
	BackgroundModeVBars2    = 10
	BackgroundModeSpectrum  = 12
	BackgroundModeBlack     = 14
)

type DiskUsage struct {
	Total uint64
	Free  uint64
	Used  uint64
}
