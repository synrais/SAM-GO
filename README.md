<div align="center">

# SAMenu
### Super Attract Menu for MiSTer FPGA

A games menu, launcher and attract mode for the [MiSTer FPGA](https://github.com/MiSTer-devel), inspired by
[**MiSTer SAM** (Super Attract Mode) by mrchrisster](https://github.com/mrchrisster/MiSTer_SAM).

Browse and search your whole collection, launch games straight into their cores with MGL files,
and let the MiSTer show itself off with an attract mode that plays random games, music and videos.

</div>

---

## Contents

- [Features](#features)
- [Installing](#installing)
- [First run](#first-run)
- [Using the menu](#using-the-menu)
  - [Controls](#controls)
  - [Systems and folders](#systems-and-folders)
  - [Search](#search)
  - [The Options menu](#the-options-menu)
- [Attract mode](#attract-mode)
  - [How games are picked](#how-games-are-picked)
  - [Attract mode controls](#attract-mode-controls)
  - [Game lists](#game-lists)
  - [Static screen detector](#static-screen-detector)
- [BIOS skip](#bios-skip)
- [Music player](#music-player)
- [Video player](#video-player)
- [Startup and idle](#startup-and-idle)
- [Command line](#command-line)
- [Configuration: SAMenu.ini](#configuration-samenuini)
  - [Name tags](#name-tags)
- [Systems and groups](#systems-and-groups)
- [Files and folders](#files-and-folders)
- [Building from source](#building-from-source)
- [Credits](#credits)

---

## Features

- 🎮 **A fast games menu** for 114 systems (consoles, handhelds, computers and arcade), grouped and sorted the way you like, with a search that works from a controller.
- 🚀 **Launches games with MGL files**, straight into the right core, including special cases like AmigaVision.
- 📺 **Attract mode** plays random games one after another, with smart picking (balanced across systems, no repeats, one version per title), a history to go back and forward through, and full control from a controller, keyboard, mouse or SSH.
- 🎲 **[Pick Random Game]** at the top of every list, and **virtual [Games A-Z] folders** for big genre-sorted sets.
- 🖥️ **A static screen detector** spots games stuck on a black or frozen screen, skips them, and learns from it.
- 🕹️ **BIOS skip** presses buttons on a virtual pad after a game loads, to get past BIOS screens.
- 🎵 **Background music**, and 🎬 **a video player**, including videos between attract games and streaming from a PC.
- ⏱️ **Startup options**: boot into SAMenu or attract mode, or start attract mode when the MiSTer's been left idle.
- 🔧 **One settings file**, `SAMenu.ini`, and almost everything in it can also be changed from the menu.
- 🔌 **Remote control** of everything over SSH, so it works with Zaparoo, Home Assistant or your own scripts.

---

## Installing

1. Copy **`SAMenu.sh`** to **`/media/fat/Scripts/`** on your MiSTer's SD card.
2. On the MiSTer, open the **Scripts** menu and run **SAMenu**.

That's it. The first time it runs, SAMenu creates its folder, `/media/fat/Scripts/.MiSTer_SAMenu/`, and its settings file, `SAMenu.ini`.

> [!NOTE]
> SAMenu is a single program with nothing else to install. MPlayer for videos, and everything else it needs, is built in.

---

## First run

The first time SAMenu opens, it **builds its games database** by scanning your games folders (`/media/fat/games`, USB drives, network shares and so on), counting each system as it goes. With a big collection this takes a few minutes. After that, SAMenu opens instantly.

Rebuild the database whenever you add or remove games: **Options → Game Database → Rebuild games database**, or `SAMenu.sh -rebuild` over SSH. Only one build runs at a time, and your old database stays usable until the new one's complete.

---

## Using the menu

### Controls

SAMenu works with a keyboard or a controller. MiSTer turns controller buttons into keys for scripts, so both work the same way.

| Controller | Keyboard | Does |
|:--|:--|:--|
| D-pad | Arrow keys | Move |
| **A** | Enter | Open, launch, choose |
| **B** | Esc | Back |
| **Y** | Tab | Search |
| **X** | Space | Options |
| **L** / **R** | Page Up / Page Down | Page up / down |

**Menu Layout** (Options → Controls) can swap A and B, **Japanese** style: B confirms, A goes back.

### Systems and folders

The first screen lists your systems. How they're grouped and sorted is up to you (**Options → Display & Sorting**): by category or manufacturer, as one list with headings or as folders you open, by release date or A-Z, and with the Arcade Cores pinned to the top if you like.

Open a system to see its games and folders. Along the way you'll find:

- **`[Pick Random Game]`** at the top of every list. It launches a random game from everything at that level and below, picked the same smart way as attract mode. (Options → Display & Sorting → Pick Random Game.)
- **`[Games A-Z]`** at the top of the systems you choose. It lists every game from all of that system's folders once, A-Z, which is perfect for a PlayStation set sorted into genres, without keeping a second copy. (Options → Display & Sorting → Virtual A-Z Folders.)
- **Disc sets**, which can be grouped into one folder per game (Options → Display & Sorting → Game list sorting).
- **Hide games tagged…**, which leaves betas, prototypes, hacks, bad dumps and so on out of the lists entirely (see [Name tags](#name-tags)).

With **Remember position** on (in Menu list options), SAMenu opens where you left off.

### Search

Press **Y** (Tab) from the systems screen, then type with the on-screen keyboard:

| Controller | Keyboard | On the search keyboard |
|:--|:--|:--|
| **A** | Enter | Type the highlighted key, or press **Search** / **Back** |
| **B** | Esc | Leave search |
| **X** | Space | Type a space |
| **Y** | Tab / Backspace | Delete the last character |
| **L** / **R** | Page Up / Page Down | Move the text cursor |

Move down past the bottom row of keys to reach the **Search** button. On a keyboard, just type and press Enter.

Results always read **`[System] Title.ext`**, with the systems A-Z and then the titles A-Z. **A** launches the highlighted game, and **B** goes back to the keyboard with your text kept.

### The Options menu

Press **X** (Space) on the systems screen.

| Section | What's in it |
|:--|:--|
| **Game Database** | Rebuild the database, and choose which systems it includes |
| **Attract Mode** | Start attract mode, its settings, and the detector and list settings |
| **Display & Sorting** | Menu list options (labels, text size, remember position), menu list sorting, game list sorting (including hidden tags), Virtual A-Z Folders and Pick Random Game |
| **Controls** | Menu Layout, the input switches, and attract mode's controls |
| **Music Player** | Play, stop and next, the playback order and the playlist |
| **Video Player** | Browse and play videos, the playlist settings, videos in attract mode, and the sync options |
| **Startup** | What starts when the MiSTer boots |

Changes are saved to `SAMenu.ini` as you make them.

---

## Attract mode

Attract mode plays random games one after another, for **`PlayTime`** seconds each (45 to 60 by default), until you pick one up and play it.

Start it from **Options → Attract Mode → Start attract mode**, over SSH with `SAMenu.sh -attract`, or automatically at boot or when the MiSTer's idle (see [Startup and idle](#startup-and-idle)).

### How games are picked

**Selection** (in Options → Attract Mode → Attract mode settings, or `[Attract] Selection`):

| Selection | How it picks |
|:--|:--|
| **Balanced** *(default)* | Systems are weighted by the square root of their size, so big libraries still come up more often but don't take over |
| **Per game** | Every game is equally likely, so systems with huge libraries dominate |
| **Per system** | A random system, then a random game, giving every system equal airtime |
| **By category** | A random category (Console, Computer…), then a system, then a game |
| **By manufacturer** | The same, with manufacturers |
| **Round robin** | Each system in turn (A-Z), with a random game from each |
| **Time machine** | Each system in turn by release date, oldest first: a tour of gaming history |
| **In order** | Every game in turn, system by system, A-Z |

Then fine-tune it:

- **No repeats**: every game plays once before any repeats.
- **Mix systems**: never the same system twice in a row.
- **One version per title**: *Tetris (USA)*, *(Europe)* and *(Japan)* count as one game. It prefers USA, then World, then Europe, clean releases before betas and hacks, and disc 1 of a multi-disc game.
- **Skip games tagged**: never pick betas, prototypes, disc 2 and so on (see [Name tags](#name-tags)).
- **Orientation**: for arcade games, Both, Horizontal, Vertical, Vertical CW or Vertical CCW, read from each MRA's `<rotation>` line. Great for a vertical cabinet.
- **Include / Exclude**: systems or groups, for example `Include = Console` and `Exclude = Nintendo`.
- **`[Weights]`**: make systems or groups come up more or less often, for example `SNES = 3`, `Computer = 0.5` or `Arcade = 0`.

**[Pick Random Game]** and `-random` pick the same way (Selection, One version per title and Weights).

### Attract mode controls

What each input does is set in **Options → Controls**, or in the `[InputDetector.*]` sections of `SAMenu.ini`. The defaults are:

| Action | Controller | Keyboard | Mouse |
|:--|:--|:--|:--|
| **next**: next game | D-pad right, stick right | Right | Right button |
| **back**: previous game (the last 100 are kept) | D-pad left, stick left | Left | Left button |
| **play**: stop attract mode and keep playing this game | Start | Enter | |
| **stop**: stop attract mode and go back to the menu | Select | Esc | |
| **blacklist**: never play this game again, next game | | Delete | |
| **stay**: stay on this game (again to carry on) | | Space | |
| **search**: type a search whose results play first (twice quickly opens SAMenu) | | `` ` `` | |
| **menu**: close the game and open SAMenu on the TV | | | |

Any other button does what **Other buttons** (`OtherInput`) says: Ignore, **Play** *(default)* or Stop.

Run **`SAMenu.sh -inputs`** over SSH to see the name of any key or button you press, for example `joystick (8BitDo Pro 2): dpleft`.

Attract mode can also **mute** the MiSTer while it runs (`Mute = true`). The sound comes back when it stops or when you choose a game.

### Game lists

Per-system lists live in `.MiSTer_SAMenu/Lists/`, one file per system, with one game title per line. The extension is optional, and lines starting with `#` or `;` are ignored.

| List | File | What it does |
|:--|:--|:--|
| **Blacklist** | `Lists/Blacklist/SNES_blacklist.txt` | Games attract mode never plays. The static detector adds black-screen games, and the **blacklist** action adds the current game |
| **Staticlist** | `Lists/Staticlist/SNES_staticlist.txt` | When each game's screen freezes, for example `<42> Game Name`. Attract mode moves on `SkipafterStatic` seconds after that point |
| **Whitelist** | `Lists/Whitelist/SNES_whitelist.txt` | A list you make yourself. For a system that has one, attract mode only plays the games on it |

Each list can be switched on or off, and limited to some systems, in `[List]`.

### Static screen detector

While attract mode plays a game, the detector watches the screen. After a **Grace** period, a screen that stays **black** or **unchanged** for too long can be added to the Blacklist or Staticlist, and skipped.

"Unchanged" is measured carefully. The screen is split into a 16×12 grid, and it only counts as moving if at least **MinSpread** cells changed in the last **SpreadTime** seconds. So a flashing cursor or a blinking *PRESS START* counts as still, while a small sprite moving around counts as moving.

To watch what it sees, live, from a PC:

```bash
ssh -t root@<mister-ip> "/media/fat/Scripts/SAMenu.sh -watch"
```

Every setting can be overridden per system or group, for example `[StaticDetector.PSX]` with a longer `Grace` for slow-loading CD games.

---

## BIOS skip

Some games stop at a BIOS or disk-system screen until a button's pressed. SAMenu plugs in a **virtual pad** (called *SAMenu*) before the game loads, presses a sequence of buttons, then unplugs it.

Sequences are set per system:

```ini
[BiosSkip.FDS]
Sequence = 10, a
```

| In a sequence | Means |
|:--|:--|
| A number | Wait that many seconds (start with one, to let the game load) |
| `a b x y l r select start home up down left right` | Virtual pad buttons (MiSTer's default map) |
| `key:enter`, `key:space`, `key:f1`… | Keyboard keys, for cores that read a keyboard |

BIOS skip can be switched on or off separately for attract mode and for games you launch (`[BiosSkip] Attract` and `Menu`). Test a sequence on a running game with `SAMenu.sh -press start`.

---

## Music player

The music player plays **MP3** and **OGG** files from **`/media/fat/music`** in the background. Each folder inside it is a **playlist**.

- Use **Options → Music Player**, or `SAMenu.sh -music start | stop | next | status`.
- **Playback** is Random or In order. **Playlist** is the music folder itself, a folder inside it, or All.
- **Pause in games** is on by default. MiSTer mixes this music into every core's sound, so it pauses while a game plays and carries on in the menu. It always pauses while a video plays.

---

## Video player

The video player plays videos from **`/media/fat/video`** on the MiSTer's own screen, using the built-in MPlayer. Folders inside it are playlists.

- **Options → Video Player → Play videos…** lets you browse and play, and **Play playlist** plays the chosen playlist.
- **In attract mode**, a video can play between games every 3, 5, 10 or 20 games (`[Video] AttractEvery`).
- **Formats**: mp4, m4v, mov, mkv, webm, avi, mpg, mpeg, ts, wmv, asf, ogv and flv.

> [!IMPORTANT]
> Videos are decoded by the MiSTer's own ARM processor. **SD video at around 480p plays well**, but 720p and above, or HEVC/H.265, is too heavy. Videos play at 640×480, and MiSTer's scaler fills the TV.

**While a video plays** (a controller works too):

| Controller | Keyboard | Does |
|:--|:--|:--|
| D-pad left / right | Left / Right | Back / forward 10 seconds |
| D-pad up / down | Up / Down | Forward / back 1 minute |
| L / R | Page Up / Page Down | Forward / back 10 minutes |
| X | Space | Pause |
| A or B | Enter or Esc | Exit |
| | 9 / 0 | Volume down / up |
| | m | Mute |

**Over SSH**:

```bash
SAMenu.sh -video play                          # the playlist set in [Video]
SAMenu.sh -video play Trailers                 # a folder (All = every video)
SAMenu.sh -video play /media/fat/video/intro.mp4
SAMenu.sh -video next | stop | status
```

**You can also stream a video straight from your PC**, with nothing copied to the MiSTer:

```bash
cat movie.mkv | ssh root@<mister-ip> "/media/fat/Scripts/SAMenu.sh -video play -"
```

MKV, TS and WebM stream best. An MP4 needs its index at the front of the file, which `ffmpeg -i in.mp4 -c copy -movflags faststart out.mp4` sorts out once.

If the sound drifts out of step after skipping, try the **sync options** in Options → Video Player.

---

## Startup and idle

**Options → Startup** sets what happens when the MiSTer boots. SAMenu writes a marked block into `/media/fat/linux/user-startup.sh` and leaves the rest of that file alone.

| Setting | Choices |
|:--|:--|
| **Start** | Nothing, **SAMenu** (boot straight into the menu) or **Attract mode** |
| **Music** | Also start the music player |
| **Attract mode starts** | **Instantly**, **After a delay** (`AttractDelay` seconds, where a press can restart the countdown, cancel it, or be ignored for kiosks), or **When idle** |

**When idle** runs a small watcher that starts attract mode once nothing's been pressed for **IdleTime** minutes, counting in the MiSTer menu, in games, or both. With games included, attract mode also comes back after you pick one of its games and put the controller down.

> [!TIP]
> SAMenu never starts attract mode while another script is running, such as `update_all`, so a core never gets loaded over an update.

If mrchrisster's MiSTer SAM is also set to start at boot, SAMenu warns you, since the two would fight over the screen.

---

## Command line

Everything can be run over SSH or from any script:

```bash
/media/fat/Scripts/SAMenu.sh <option>
```

| Option | Does |
|:--|:--|
| *(none)* | Open SAMenu |
| **Attract mode** | |
| `-attract` | Start attract mode (its log on screen) |
| `-attract -bg` | Start it in the background (log in `/tmp/SAMenu_attract.log`) |
| `-attract SNES,Console` | Only these systems or groups, this time |
| `-status` | Is it running, and what's playing? |
| `-next` / `-back` | Next or previous game |
| `-play` | Stop attract mode and keep playing this game |
| `-stay` | Stay on this game (again to carry on) |
| `-blacklist` | Never play this game again, next game |
| `-stop` | Stop attract mode and go back to the MiSTer menu |
| `-find mario 3` | Play the games matching all the words first, then carry on |
| **SAMenu on the TV** | |
| `-menu` | Open SAMenu on the TV (closes the running game) |
| `-search` | …straight into its Search screen |
| **Games** | |
| `-launch <file>` | Launch a game, for example `-launch /media/fat/games/NES/Tetris.nes` |
| `-random` | Launch a random game |
| `-random Nintendo` | …from these systems or groups |
| `-list` | Print every game in the database |
| `-rebuild` | Rebuild the games database |
| **Music and video** | |
| `-music start \| stop \| next \| status` | The music player |
| `-video play [file \| folder \| -]` | The video player (`-` plays a video piped in) |
| `-video next \| stop \| status` | Control the video player |
| **Testing** | |
| `-inputs` | Print every key, click and controller press |
| `-press start` | Press buttons on the virtual pad |
| `-watch` | Show the static detector's live status |

`-launch`, `-random` and `-video play` end a running attract mode first. The other options never disturb a running menu or attract mode.

That makes SAMenu easy to drive from **Zaparoo**, **Home Assistant** or your own scripts.

---

## Configuration: SAMenu.ini

All settings live in **`/media/fat/Scripts/.MiSTer_SAMenu/SAMenu.ini`**. Every setting is explained in the file itself, and the top of the file has a command cheat sheet plus the full list of system IDs and groups. Most settings can be changed from the Options menu instead.

Settings are case-insensitive, and lists are comma separated. Anywhere a system is accepted, a **group** works too (see [Systems and groups](#systems-and-groups)).

<details>
<summary><b>[Systems]</b>: extra games folders and alternative cores</summary>

| Key | Example | Does |
|:--|:--|:--|
| `games_folder` | `/media/usb0/MoreGames` | An extra place laid out like a `games` folder, with a folder per system inside. One line each |
| `system_folder` | `SNES:/media/usb0/SNES Hacks` | An extra folder for one system, used as it is. One line each |
| `set_core` | `SNES:_Console/SNES_alt` | Use a different core for a system |

</details>

<details>
<summary><b>[Database]</b> and <b>[Database.X]</b>: what goes into the games database</summary>

`[Database] Exclude` leaves whole systems or groups out, for example `Exclude = Computer, NESMusic`.

`[Database.X]` sections leave out only some games, where **X** is `ALL`, a system ID or a group:

| Key | Matches |
|:--|:--|
| `Folders` | Folder names inside a system |
| `Files` | File names (the extension is optional) |
| `Extensions` | Extensions, with or without the dot |
| `Paths` | A location and everything under it, for example `/media/usb1` |

Patterns ignore case: `hack` matches exactly, `hack*` starts with, `*hack` ends with, and `*hack*` contains.

```ini
[Database.ALL]
Files = *(Proto)*, *(Beta)*

[Database.SNES]
Folders    = Hacks
Extensions = bs
```

Rebuild the database after changing these.

</details>

<details>
<summary><b>[Attract]</b>, <b>[Weights]</b> and <b>[Attract.X]</b>: attract mode</summary>

| Key | Default | Does |
|:--|:--|:--|
| `PlayTime` | `45-60` | Seconds per game: a number, or a range to pick from |
| `Selection` | `Balanced` | How games are picked (see [How games are picked](#how-games-are-picked)) |
| `NoRepeats` | `true` | Every game once before any repeats |
| `MixSystems` | `true` | Never the same system twice in a row |
| `OneVersion` | `true` | One version per title |
| `SkipTags` | *(empty)* | [Name tags](#name-tags) never to pick |
| `Orientation` | `Both` | Arcade: Both, Horizontal, Vertical, Vertical CW or Vertical CCW |
| `Include` / `Exclude` | *(empty)* | Systems or groups (Exclude wins) |
| `UseStaticDetector` | `true` | Run the static screen detector |
| `OtherInput` | `Play` | What unbound buttons do: Ignore, Play or Stop |
| `Mute` | `false` | Mute the MiSTer while attract mode plays |

`[Weights]` takes `name = number`, for example `SNES = 3`, `Computer = 0.5`, or `Arcade = 0` for never.

`[Attract.X]` uses the same Folders / Files / Extensions / Paths rules as `[Database.X]`, but only for attract mode, with no rebuild needed.

</details>

<details>
<summary><b>[List]</b>: Blacklist, Staticlist and Whitelist</summary>

| Key | Default | Does |
|:--|:--|:--|
| `UseBlacklist` | `true` | Never play blacklisted games |
| `UseStaticlist` | `true` | Leave games `SkipafterStatic` seconds after they freeze |
| `SkipafterStatic` | `10` | Seconds |
| `UseWhitelist` | `false` | Only play whitelisted games (for systems with a whitelist) |
| `…Include` / `…Exclude` | *(empty)* | Limit each list to some systems or groups |

</details>

<details>
<summary><b>[StaticDetector]</b>: the static screen detector</summary>

| Key | Default | Does |
|:--|:--|:--|
| `Grace` | `25` | Seconds before anything counts |
| `BlackThreshold` | `30` | Seconds of black screen before it acts |
| `StaticThreshold` | `30` | Seconds of still screen before it acts |
| `SkipBlack` / `SkipStatic` | `true` | Skip to the next game |
| `WriteBlackList` / `WriteStaticList` | `true` | Add the game to the list |
| `MinSpread` | `8` | Grid cells (out of 192) that must change to count as moving |
| `SpreadTime` | `5` | …within this many seconds |

Override any of these per system or group, for example `[StaticDetector.PSX]`. A system's own section wins over a group's.

</details>

<details>
<summary><b>[InputDetector]</b> and <b>[InputDetector.Keyboard / Mouse / Joystick]</b>: attract mode controls</summary>

`[InputDetector]` switches each kind of input on or off: `Mouse`, `Keyboard` and `Joystick`.

The other sections take `input = action`, where the actions are `next`, `back`, `play`, `stop`, `blacklist`, `stay`, `menu` and `search`:

```ini
[InputDetector.Joystick]
dpleft  = back
dpright = next
start   = play
back    = stop
```

Use `SAMenu.sh -inputs` to find the names. Controllers the SDL controller database doesn't know use raw names like `btn0` and `axis0+`.

</details>

<details>
<summary><b>[BiosSkip]</b> and <b>[BiosSkip.X]</b>: BIOS skip</summary>

| Key | Default | Does |
|:--|:--|:--|
| `Attract` | `true` | After attract mode launches a game |
| `Menu` | `true` | After you launch a game from SAMenu |

`[BiosSkip.<system ID>]` takes `Sequence = ...`, see [BIOS skip](#bios-skip).

</details>

<details>
<summary><b>[Music]</b>, <b>[Video]</b> and <b>[Startup]</b></summary>

**[Music]**

| Key | Default | Does |
|:--|:--|:--|
| `Playback` | `Random` | Random or In order |
| `Playlist` | *(empty)* | Empty for the music folder itself, `All`, or a folder name |
| `PauseInGames` | `true` | Pause while a game plays |

**[Video]**

| Key | Default | Does |
|:--|:--|:--|
| `Playback` | `Random` | Random or In order |
| `Playlist` | *(empty)* | Empty for the video folder itself, `All`, or a folder name |
| `AttractEvery` | `0` | A video every this many attract games (0 = never) |
| `AutoSync` | `true` | Correct audio/video drift quickly |
| `CorrectPts` | `false` | MP4 and MKV: use the file's own timestamps |
| `Mp3Seek` | `false` | Files with MP3 audio: accurate seeking |
| `AviIndex` | `false` | AVIs without an index: build one first |

**[Startup]**

| Key | Default | Does |
|:--|:--|:--|
| `Start` | `Nothing` | Nothing, SAMenu or Attract mode |
| `Music` | `false` | Start the music player |
| `AttractWhen` | `Instantly` | Instantly, After a delay, or When idle |
| `AttractDelay` | `60` | Seconds, for After a delay |
| `AttractPress` | `Restarts the countdown` | Or Cancels it, or Is ignored |
| `IdleTime` | `5` | Minutes, for When idle |
| `IdleWhere` | `MiSTer menu` | MiSTer menu, Games, or Menu+Games |

After editing these by hand, save once from Options → Startup so the startup block gets updated.

</details>

<details>
<summary><b>[Menu]</b> and <b>[Controls.Menu]</b>: how the menu looks and sorts</summary>

**Labels**

| Key | Default | Does |
|:--|:--|:--|
| `LabelAlign` | `Left` | Left, Center or Right |
| `LabelDivider` | `Space` | Space, Dash, Colon, Pipe or Arrow |
| `LabelAlignDivider` | `false` | Line the dividers up in one column |
| `LabelEncapsulate` | `[ ]` | Around the manufacturer: None, `[ ]`, `( )`, `< >` or `{ }` (also spaced) |
| `ShowManufacturer` | `true` | Off shows just the system's full name |
| `TextSize` | `Auto` | Auto, Normal, Large, Extra large or Huge |
| `RememberPosition` | `false` | Open where you left off |

**Systems list**

| Key | Default | Does |
|:--|:--|:--|
| `SystemGroup` | `Category` | Manufacturer, Category or None |
| `SystemGroupOrder` | `Custom` | A-Z, Oldest first, or Custom |
| `CategoryOrder` | `Arcade, Console, Handheld, Computer, Other` | The order used by Custom |
| `SystemSubgroup` | `Manufacturer` | A second level of groups |
| `GroupView` | `Folders` | List or Folders |
| `GroupHeaders` | `Off` | With List: Off, Line, Double, Brackets, Dots or Minimal |
| `SystemOrder` | `Release date` | Or A-Z |
| `ArcadeFirst` | `false` | Pin the Arcade Cores to the top |

**Game lists**

| Key | Default | Does |
|:--|:--|:--|
| `FolderPosition` | `First` | First, Last or Mixed |
| `GameOrder` | `Natural` | Alphabetical, or Natural ("Game 2" before "Game 10") |
| `IgnoreThe` | `false` | Sort "The Legend of Zelda" under L |
| `HideExtensions` | `false` | Hide file extensions |
| `GroupDiscs` | `false` | One folder per multi-disc game |
| `VirtualAZFolders` | *(empty)* | Systems that get a `[Games A-Z]` folder |
| `RandomEntry` | `true` | Show `[Pick Random Game]` |
| `HideTags` | *(empty)* | [Name tags](#name-tags) to hide from the lists |
| `SearchHidden` | `true` | Hide them from search too |

**[Controls.Menu]** has `Layout = Western` (A confirms) or `Japanese` (B confirms).

</details>

### Name tags

`SkipTags` (attract mode) and `HideTags` (the menu) take these tags, which you can also tick in a list in the menu:

| Tag | Matches |
|:--|:--|
| Beta | `(Beta)`, `(Beta 2)` |
| Proto | `(Proto)`, `(Prototype)` |
| Demo | `(Demo)`, `(Sample)`, `(Kiosk)`, `(Promo)`, `(Preview)` |
| Program | `(Program)`, `(Test Program)`, `[BIOS]` |
| Hack | `(Hack)`, `[h]`, `[h1]` |
| Translation | `[T+Eng]`, `[T-Eng]`, and `T+Eng` without brackets |
| Unlicensed | `(Unl)`, `(Pirate)`, `(Bootleg)` |
| Homebrew | `(Homebrew)`, `(Aftermarket)` |
| Cheat / Trainer / Fixed | `[c]`, `[t]`, `[f]` |
| Bad dump | `[b]`, `[b1]`, `[o]` |
| Alternate | `[a]`, `[a1]` |
| Disc 2+ | `(Disc 2)` and up, `(Side B)`, `(Tape 2)` |
| Re-releases | `(Virtual Console)`, `(Switch Online)`, `(Collection)`, `(Mini)` |

Tags only match inside `( )` or `[ ]`, as whole words, so **Demo** never catches *Demolition Man*.

> [!WARNING]
> If you hide **Disc 2+** in the menu, you can't pick disc 2 from the menu when a game asks for it, unless your multi-disc games use `.m3u` playlists.

---

## Systems and groups

SAMenu knows **114 systems**. Their IDs (like `SNES`, `MegaDrive`, `PSX` and `Arcade`) are listed at the top of `SAMenu.ini`.

Anywhere a system ID is accepted (Include, Exclude, Weights, the list and detector sections, `-attract` and `-random`), you can also use a **group**:

- **Categories**: `Console`, `Handheld`, `Computer`, `Arcade` and `Other`
- **Manufacturers**: `Nintendo`, `Sega`, `Sony`, `Atari`, `Commodore`, `SNK`, `NEC` and many more, all listed in `SAMenu.ini`

For example, `Include = Nintendo, Sega` with `Exclude = Gameboy2P` plays every Nintendo and Sega system except the two-player Game Boy core.

---

## Files and folders

| Where | What |
|:--|:--|
| `/media/fat/Scripts/SAMenu.sh` | The program |
| `/media/fat/Scripts/.MiSTer_SAMenu/SAMenu.ini` | The settings |
| `/media/fat/Scripts/.MiSTer_SAMenu/games.db` | The games database |
| `/media/fat/Scripts/.MiSTer_SAMenu/Lists/` | The Blacklist, Staticlist and Whitelist |
| `/media/fat/Scripts/.MiSTer_SAMenu/position.txt` | Where you left off (Remember position) |
| `/media/fat/music/` | Music (folders are playlists) |
| `/media/fat/video/` | Videos (folders are playlists) |
| `/tmp/SAMenu_attract.log` | Attract mode's log, when started with `-bg` |
| `/tmp/SAMenu_attract.status` | What attract mode is playing |
| `/tmp/SAMenu_detector` | The static detector's live status |

To use a different settings file, set the environment variable `SAMENU_CONFIG`.

---

## Building from source

SAMenu is written in Go and uses ncurses, so it's built as a static ARM binary with a cross-compiler. From Linux or WSL:

```bash
# one-time setup
sudo apt install gcc-arm-linux-gnueabihf libncurses-dev:armhf

# build
CGO_ENABLED=1 GOOS=linux GOARCH=arm GOARM=7 CC=arm-linux-gnueabihf-gcc \
PKG_CONFIG_LIBDIR=/usr/lib/arm-linux-gnueabihf/pkgconfig \
go build -trimpath -tags netgo,osusergo \
  -ldflags '-s -w -linkmode external -extldflags "-static -lncurses -ltinfo"' \
  -o SAMenu.sh ./cmd/samenu
```

On Windows, `build_deploy_SAMenu.bat` runs the build in WSL and copies the result to the MiSTer. Set your MiSTer's IP at the top; it also needs `sshpass`. The other `.bat` files run things on the MiSTer over SSH: `start_attract.bat`, `watch_detector.bat`, `test_inputs.bat` and `test_press.bat`.

---

## Credits

- [**MiSTer SAM**](https://github.com/mrchrisster/MiSTer_SAM) by mrchrisster and contributors: the original Super Attract Mode, and the inspiration for SAMenu.
- The [**MiSTer FPGA project**](https://github.com/MiSTer-devel) and all its core developers.
- [**MPlayer**](http://www.mplayerhq.hu/) for video playback, and the [**SDL GameController DB**](https://github.com/mdqinc/SDL_GameControllerDB) for controller names.
