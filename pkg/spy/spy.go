package spy

/*
#cgo CFLAGS: -I${SRCDIR}
#cgo LDFLAGS: -L${SRCDIR} -lspy -static
#include "spy.h"
*/
import "C"

// Run starts the C-based spy monitor
func Run() {
	C.spy_scan_devices()
	C.spy_loop()
}
