package config

const UserConfigEnv = "SAM_CONFIG"
const UserAppPathEnv = "SAM_APP_PATH"

const ActiveGameFile = TempFolder + "/ACTIVEGAME"

const PidFileTemplate = TempFolder + "/%s.pid"
const LogFileTemplate = TempFolder + "/%s.log"

const ScriptsConfigFolder = ScriptsFolder + "/.config"
const SAMConfigFolder = ScriptsConfigFolder + "/sam"

const LastLaunchFile = "/tmp/.LASTLAUNCH.mgl"

// MenuDb is the games database shared by SAM, the games menu and search.
const MenuDb = SAMFolder + "/games.db"

// oldMenuDb is where the games database used to live; config.Load moves it.
const oldMenuDb = SAMConfigFolder + "/menu.db"

// ListsFolder holds the per-system Blacklist, Staticlist and Whitelist files.
const ListsFolder = SAMFolder + "/Lists"
