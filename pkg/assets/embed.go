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

//go:embed default.ini
var defaultINI []byte

//go:embed gamecontrollerdb.txt
var GameControllerDB string

//go:embed keyboardscancodes.txt
var KeyboardScanCodes string

//go:embed SAM.ini
var DefaultSAMIni []byte

//go:embed sidelaunchers/AmigaVision.zip
var AmigaVisionZip []byte

//go:embed sidelaunchers/AmigaCD32.zip
var AmigaCD32Zip []byte

// --- Helpers ---

// ExtractZip extracts an embedded zip (like AmigaVisionZip) into destDir.
// It forces +x (execute) permission on extracted files to ensure binaries run.
func ExtractZip(data []byte, destDir string) error {
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

		if err := extractAndWrite(f, fpath); err != nil {
			return err
		}
	}
	return nil
}

// ExtractZipFile extracts a single file from an embedded zip into destPath.
func ExtractZipFile(data []byte, fileName, destPath string) error {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}

	for _, f := range r.File {
		if f.Name == fileName {
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				return err
			}
			return extractAndWrite(f, destPath)
		}
	}
	return os.ErrNotExist
}

// Internal helper for file extraction
func extractAndWrite(f *zip.File, destPath string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	dst, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, rc); err != nil {
		return err
	}

	// Force +x on extracted files (safety for any binaries/scripts inside)
	return os.Chmod(destPath, f.Mode()|0111)
}
