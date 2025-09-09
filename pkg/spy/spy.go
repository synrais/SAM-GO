package spy

/*
#cgo CFLAGS: -I.
#cgo LDFLAGS: -L. -lspy
#include "spy.h"
*/
import "C"

// Run starts the C-based spy monitor
func Run() {
	C.spy_scan_devices()
	C.spy_loop()
}
