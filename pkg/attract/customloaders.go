func init() {
	RegisterCustomLoader("AmigaCD32", func(system *games.System, runPath string) error {
		fmt.Println("[AmigaCD32] Custom loader starting…")

		// Ensure temp working dir
		tmpDir := "/tmp/.SAM_tmp/AmigaCD32"
		if err := os.MkdirAll(tmpDir, 0755); err != nil {
			return fmt.Errorf("failed to create temp dir: %w", err)
		}

		// Decide pseudoRoot
		pseudoRoot := ""
		saveFile := "AmigaVision-Saves.hdf"
		foundSave := ""
		cfg := &config.UserConfig{}

		for _, pathResult := range games.GetSystemPaths(cfg, []games.System{*system}) {
			candidate := filepath.Join(pathResult.Path, saveFile)
			if _, err := os.Stat(candidate); err == nil {
				foundSave = candidate
				pseudoRoot = pathResult.Path
				break
			}
		}
		if pseudoRoot == "" {
			// fallback: first system folder
			paths := games.GetSystemPaths(cfg, []games.System{*system})
			if len(paths) > 0 {
				pseudoRoot = paths[0].Path
			} else {
				return fmt.Errorf("could not determine pseudoRoot for AmigaCD32")
			}
		}
		fmt.Printf("[AmigaCD32] Using pseudoRoot = %s\n", pseudoRoot)

		// 1. Write embedded blank cfg to tmp
		tmpCfg := filepath.Join(tmpDir, "AmigaCD32.cfg")
		if err := os.WriteFile(tmpCfg, assets.BlankAmigaCD32Cfg, 0644); err != nil {
			return fmt.Errorf("failed to write temp cfg: %w", err)
		}

		// 2. Resolve game path and strip /media prefix
		absGame, err := filepath.Abs(runPath)
		if err != nil {
			return fmt.Errorf("failed to resolve game path: %w", err)
		}
		if strings.HasPrefix(absGame, "/media/") {
			absGame = absGame[len("/media/"):]
		}

		// 3. Patch config placeholders
		cfgData, err := os.ReadFile(tmpCfg)
		if err != nil {
			return err
		}
		patches := map[string]string{
			"gamepath.ext": absGame,
			"/AGS-SAVES.hdf": filepath.Join(pseudoRoot, "AmigaVision-Saves.hdf"),
			"/CD32.hdf":      filepath.Join(pseudoRoot, "AmigaCD32.hdf"),
			"/AGS.rom":       filepath.Join(pseudoRoot, "AmigaVision.rom"),
		}
		for placeholder, val := range patches {
			idx := bytes.Index(cfgData, []byte(placeholder))
			if idx == -1 {
				continue
			}
			copy(cfgData[idx:], []byte(val))
		}
		if err := os.WriteFile(tmpCfg, cfgData, 0644); err != nil {
			return err
		}
		fmt.Printf("[AmigaCD32] Patched cfg written to %s\n", tmpCfg)

		// 4. Write embedded assets into tmp
		if err := os.WriteFile(filepath.Join(tmpDir, "AmigaCD32.hdf"), assets.AmigaCD32Hdf, 0644); err != nil {
			return fmt.Errorf("failed to write AmigaCD32.hdf: %w", err)
		}
		if err := os.WriteFile(filepath.Join(tmpDir, "AmigaVision.rom"), assets.AmigaVisionRom, 0644); err != nil {
			return fmt.Errorf("failed to write AmigaVision.rom: %w", err)
		}

		// 5. Bind mounts
		mounts := [][2]string{
			{tmpCfg, "/media/fat/config/AmigaCD32.cfg"},
			{filepath.Join(tmpDir, "AmigaCD32.hdf"), filepath.Join(pseudoRoot, "AmigaCD32.hdf")},
			{filepath.Join(tmpDir, "AmigaVision.rom"), filepath.Join(pseudoRoot, "AmigaVision.rom")},
		}
		for _, m := range mounts {
			cmd := exec.Command("mount", "--bind", m[0], m[1])
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("failed bind-mount %s -> %s: %w", m[0], m[1], err)
			}
		}
		fmt.Println("[AmigaCD32] Assets and cfg bind-mounted")

		// 6. Build the special MGL
		mgl := `<mistergamedescription>
	<rbf>_computer/minimig</rbf>
	<setname same_dir="1">AmigaCD32</setname>
</mistergamedescription>`

		tmpMgl := config.LastLaunchFile
		if err := os.WriteFile(tmpMgl, []byte(mgl), 0644); err != nil {
			return fmt.Errorf("failed to write custom MGL: %w", err)
		}

		// 7. Launch
		if err := mister.LaunchGenericFile(cfg, tmpMgl); err != nil {
			return fmt.Errorf("failed to launch AmigaCD32 MGL: %w", err)
		}

		fmt.Println("[AmigaCD32] Game launched successfully!")
		return nil
	})
}
