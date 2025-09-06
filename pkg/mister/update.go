package mister

import (
	"github.com/synrais/SAM-GO/pkg/config"
	"os"
	"time"
)

func GetLastUpdateTime() (time.Time, error) {
	file, err := os.Stat(config.DownloaderLastRun)
	if os.IsNotExist(err) {
		return time.Time{}, nil
	} else if err != nil {
		return time.Time{}, err
	}

	return file.ModTime(), nil
}
