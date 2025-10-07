package config

const UserConfigEnv  = "SAM_CONFIG"
const UserAppPathEnv = "SAM_APP_PATH"

const ActiveGameFile  = TempFolder + "/ACTIVEGAME"
const SearchDbFile    = SdFolder + "/search.db"
const PlayLogDbFile   = SdFolder + "/playlog.db"

const PidFileTemplate = TempFolder + "/%s.pid"
const LogFileTemplate = TempFolder + "/%s.log"

const ScriptsConfigFolder = ScriptsFolder + "/.config"
const SAMConfigFolder     = ScriptsConfigFolder + "/sam"

const ArcadeDBUrl  = "https://api.github.com/repositories/521644036/contents/ArcadeDatabase_CSV"
const ArcadeDBFile = SAMConfigFolder + "/ArcadeDatabase.csv"

const NfcDatabaseFile = SdFolder + "/nfc.csv"
const NfcLastScanFile = TempFolder + "/NFCSCAN"

const LastLaunchFile = "/tmp/.LASTLAUNCH.mgl"

const MenuDb = SAMConfigFolder + "/menu.db"
