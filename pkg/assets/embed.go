package assets

import (
	_ "embed" // Required to embed files
)

//go:embed gamecontrollerdb.txt
var GameControllerDB string

//go:embed keyboardscancodes.txt
var KeyboardScanCodes string

//go:embed SAM.ini
var DefaultSAMIni []byte
