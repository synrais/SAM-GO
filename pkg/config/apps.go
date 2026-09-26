package config

const UserConfigEnv = "SAMENU_CONFIG"
const UserAppPathEnv = "SAMENU_APP_PATH"

const ActiveGameFile = TempFolder + "/ACTIVEGAME"

const PidFileTemplate = TempFolder + "/%s.pid"
const LogFileTemplate = TempFolder + "/%s.log"

const ScriptsConfigFolder = ScriptsFolder + "/.config"

const LastLaunchFile = "/tmp/.LASTLAUNCH.mgl"

// MenuDb is the games database shared by SAMenu's menu, attract mode and search.
const MenuDb = SAMFolder + "/games.db"

// ListsFolder holds the per-system Blacklist, Staticlist and Whitelist files.
const ListsFolder = SAMFolder + "/Lists"
