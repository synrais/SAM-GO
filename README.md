# SAM-GO
SAM ported to GO
SAM -list <flags>
-----------

-o <dir>         Output directory for gamelist files
                 Default: "."

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

SAM -joystickAS
-----------
Starts the joystick watcher

SAM -keyboard
-----------
Starts the keyboard watcher
