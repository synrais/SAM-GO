package assets

import (
    _ "embed" // Required to embed files
)

//go:embed gamecontrollerdb.txt
var GameControllerDB string
