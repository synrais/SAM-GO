package main

import (
    "fmt"
    "os"
    "os/exec"
    "path/filepath"

    "github.com/synrais/SAM-GO/pkg/assets"
)

const playerDir = "/tmp/mrext-mplayer"

func setupPlayer() error {
    if err := os.MkdirAll(playerDir, 0755); err != nil {
        return err
    }
    return assets.ExtractZipBytes(assets.MPlayerZip, playerDir)
}

func runMplayer(path string) error {
    playerPath := filepath.Join(playerDir, "mplayer")
    cmd := exec.Command(playerPath, path)
    cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+playerDir)
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    return cmd.Run()
}

func main() {
    if len(os.Args) < 2 {
        fmt.Println("Usage: testplayer <moviefile>")
        os.Exit(1)
    }

    if err := setupPlayer(); err != nil {
        panic(err)
    }

    if err := runMplayer(os.Args[1]); err != nil {
        panic(err)
    }
}
