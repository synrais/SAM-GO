# SAM-GO
A modern Go port of **SAM** (Super Attract Mode) for MiSTer.  
Fast, memory-efficient, and fully integrated with Go’s concurrency model.

---

## 🚀 Usage

### Start Attract Mode
```bash
SAM
```
Runs Attract Mode, cycling through your games automatically.

---

### Start Attract Mode with Static Detector Debug
```bash
SAM -s
```
Same as Attract Mode, but with live debug output from the static detector.

---

### Run a Single Game
```bash
SAM -run /full/path/to/game
```
Launches a single game directly and updates `Now_Playing.txt`.  
Useful for integrating with scripts or quick testing.

---

### Launch Interactive Game Browser Menu
```bash
SAM -menu
```
Opens the new **Go-based interactive menu**, letting you browse by system and game.  
No shell scripts required — runs entirely within SAM-GO.

---

## ⚡️ Features

- **Unified caching**: all gamelists, masterlist, and index handled consistently in RAM.  
- **History navigation**: step back/forward through previously played titles.  
- **Search mode**: fast in-RAM search across all indexed games (ignores system header lines automatically).  
- **Now Playing integration**: writes `/tmp/Now_Playing.txt` for external tools.  
- **MiSTer integration**: launches games directly via the MiSTer command system.  
- **Lightweight**: ~128MB memory limit, built for efficiency.

---

## 📂 Gamelist Storage

- Gamelists and metadata live under:
  ```
  /media/fat/Scripts/.MiSTer_SAM/SAM_Gamelists
  ```
- On startup, SAM-GO:
  - Loads `MasterList` and `GameIndex` into RAM
  - Generates system gamelists (`*_gamelist.txt`)
  - Applies filters (include/exclude)
  - Keeps everything in memory for instant access

---

## 🛠 Development Notes

- **Attract Mode (`SAM`, `SAM -s`)**: main event loop, auto-cycling games.
- **Menu Mode (`SAM -menu`)**: interactive menu (port of old `SAM_MENU.sh`).
- **Run Mode (`SAM -run <path>`)**: launches directly into a game.
- **Search**: in-RAM search engine (prefix, substring, fuzzy).

---

## ✅ Example Workflows

Start Attract Mode:
```bash
SAM
```

Run with debug stream:
```bash
SAM -s
```

Jump straight into Sonic 2:
```bash
SAM -run /media/fat/games/Genesis/Sonic2.bin
```

Browse all NES games via menu:
```bash
SAM -menu
```
