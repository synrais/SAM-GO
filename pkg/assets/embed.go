package assets

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"io"
	"os"
	"path/filepath"
)

// --- Embeds ---

//go:embed gamecontrollerdb.txt
var GameControllerDB string

//go:embed keyboardscancodes.txt
var KeyboardScanCodes string

//go:embed SAM.ini
var DefaultSAMIni []byte

//go:embed Blank_AmigaCD32.cfg
var BlankAmigaCD32Cfg []byte

//go:embed mplayer.zip
var MPlayerZip []byte

// --- Helpers ---

// ExtractZipBytes extracts an embedded zip (like MPlayerZip) into destDir.
// It also forces +x (execute) permission on extracted files to ensure binaries run.
func ExtractZipBytes(data []byte, destDir string) error {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}

	for _, f := range r.File {
		fpath := filepath.Join(destDir, f.Name)

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fpath, f.Mode()); err != nil {
				return err
			}
			continue
		}

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		dst, err := os.OpenFile(fpath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}

		if _, err := io.Copy(dst, rc); err != nil {
			dst.Close()
			rc.Close()
			return err
		}

		dst.Close()
		rc.Close()

		// Force +x on extracted files to avoid permission denied
		if err := os.Chmod(fpath, f.Mode()|0111); err != nil {
			return err
		}
	}
	return nil
}
