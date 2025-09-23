# SAM-GO
SAM ported to GO

## Running multiple commands

SAM can execute several commands in parallel. Separate command groups with
`--` and every group will run in its own goroutine:

```
SAM -list -s NES -- -run SuperMario -- -attract
```

If an instance is already running, the commands are forwarded to it through the
UNIX socket at `/tmp/sam.sock`. The socket also accepts multiple commands in a
single request when each line contains a command with its arguments separated by
NUL characters (`\x00`).

SAM
-----------
Starts Attract Mode

SAM -s
-----------
Starts Attract Mode with staticdetector debug output

Following commands are depreciated

-----------
SAM -list <flags>
-----------

-o <dir>         Output directory for gamelist files
                 Default: "/media/fat/Scripts/.MiSTer_SAM/SAM_Gamelists"

-s <systems>     List of systems to index (comma separated)
                 Default: "all"
                 Example: -s NES,SNES,Genesis

-p               Print output formatted for a dialog gauge
                 Default: false

-q               Suppress all status output (quiet mode)
                 Default: false

-d               List active system folders (no gamelists built)
                 Default: false

-nodupes         Filter out duplicate games (prefers .mgl files)
                 Default: false
				 
-overwrite		 Overwrite exisiting gamelists (instead of skipping)
				 Default: false
				 
-exclude <sys>   Exclude systems from scanning/list building
                 Comma separated list of system IDs
                 Default: none

SAM -run <target>
-----------

  <target> can be:
    • An AmigaVision game name (string without slashes)
    • An .mgl file path
    • A generic file path (ROM, etc.)
	
	
SAM -attract
-----------
Starts Attract Mode

SAM -mouse
-----------
Starts the mouse watcher

SAM -joystick
-----------
Starts the joystick watcher

SAM -keyboard
-----------
Starts the keyboard watcher
