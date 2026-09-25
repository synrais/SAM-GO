package input

// HID report descriptors
//
// Keyboards and mice are read through hidraw, the raw USB/Bluetooth data,
// because MiSTer holds the normal input devices exclusively while a core
// is running (the Linux "grab"), and hidraw isn't affected by that.
//
// Every HID device describes the layout of its own data in a "report
// descriptor". Reading it (instead of assuming the simplest keyboard
// layout) means keyboards with report IDs, N-key rollover keyboards,
// combo keyboard/mouse receivers and ordinary mice all decode correctly.

const (
	pageGenericDesktop = 0x01
	pageKeyboard       = 0x07
	pageButton         = 0x09

	usageMouse    = 0x02
	usageKeyboard = 0x06
	usageX        = 0x30
	usageY        = 0x31
	usageWheel    = 0x38
)

// hidField is one Input item: Count values of Size bits each.
type hidField struct {
	ReportID   int
	BitOffset  int // from the start of the report data, after any ID byte
	Size       int
	Count      int
	Variable   bool // one value per usage (bits, axes), else an array of usages
	Relative   bool
	LogMin     int32
	Page       uint16
	Usages     []uint16 // explicit usages, one per value (Variable fields)
	UsageMin   uint16   // usage range, for arrays or when no usages are listed
	UsageMax   uint16
	AppUsage   uint32 // page<<16 | usage of the application collection
	HasUsages  bool
	usageRange bool
}

// usageAt returns the usage of value i of a Variable field.
func (f *hidField) usageAt(i int) (uint16, bool) {
	if len(f.Usages) > 0 {
		if i < len(f.Usages) {
			return f.Usages[i], true
		}
		return f.Usages[len(f.Usages)-1], true // HID: last usage repeats
	}
	if f.usageRange {
		u := int(f.UsageMin) + i
		if u <= int(f.UsageMax) {
			return uint16(u), true
		}
	}
	return 0, false
}

// hidLayout is a parsed report descriptor.
type hidLayout struct {
	Fields     []hidField
	UsesIDs    bool // reports start with a report ID byte
	isKeyboard bool
	isMouse    bool
}

// parseHIDDescriptor reads a report descriptor. It understands the items
// keyboards and mice use; anything else is skipped safely.
func parseHIDDescriptor(d []byte) *hidLayout {
	type globals struct {
		page       uint16
		logMin     int32
		reportSize int
		reportID   int
		count      int
	}
	var (
		g        globals
		stack    []globals
		usages   []uint16
		usePages []uint16 // page of each usage, when given as 4 bytes
		uMin     uint32
		uMax     uint32
		haveMin  bool
		haveMax  bool
		offsets  = map[int]int{} // report ID -> next input bit
		apps     []uint32        // collection stack (0 = not an application)
		layout   = &hidLayout{}
	)
	clearLocals := func() {
		usages, usePages = usages[:0], usePages[:0]
		haveMin, haveMax = false, false
	}
	currentApp := func() uint32 {
		for i := len(apps) - 1; i >= 0; i-- {
			if apps[i] != 0 {
				return apps[i]
			}
		}
		return 0
	}

	for i := 0; i < len(d); {
		prefix := d[i]
		if prefix == 0xFE { // long item: skip
			if i+1 >= len(d) {
				break
			}
			i += 3 + int(d[i+1])
			continue
		}
		size := int(prefix & 3)
		if size == 3 {
			size = 4
		}
		typ := (prefix >> 2) & 3
		tag := prefix >> 4
		if i+1+size > len(d) {
			break
		}
		var u uint32
		for b := 0; b < size; b++ {
			u |= uint32(d[i+1+b]) << (8 * b)
		}
		// signed version of the data
		s := int32(u)
		switch size {
		case 1:
			s = int32(int8(u))
		case 2:
			s = int32(int16(u))
		}
		i += 1 + size

		switch typ {
		case 0: // main
			switch tag {
			case 0x8: // Input
				flags := u
				f := hidField{
					ReportID:  g.reportID,
					BitOffset: offsets[g.reportID],
					Size:      g.reportSize,
					Count:     g.count,
					Variable:  flags&0x02 != 0,
					Relative:  flags&0x04 != 0,
					LogMin:    g.logMin,
					Page:      g.page,
					AppUsage:  currentApp(),
				}
				offsets[g.reportID] += g.reportSize * g.count
				if flags&0x01 != 0 { // constant: padding
					break
				}
				if len(usages) > 0 {
					f.Usages = append([]uint16(nil), usages...)
					if len(usePages) > 0 && usePages[0] != 0 {
						f.Page = usePages[0]
					}
				}
				if haveMin && haveMax {
					f.usageRange = true
					f.UsageMin, f.UsageMax = uint16(uMin), uint16(uMax)
					if uMin>>16 != 0 {
						f.Page = uint16(uMin >> 16)
					}
				}
				f.HasUsages = len(f.Usages) > 0 || f.usageRange
				if f.HasUsages && f.Size > 0 && f.Count > 0 {
					layout.Fields = append(layout.Fields, f)
				}
			case 0xA: // Collection
				app := uint32(0)
				if u == 0x01 && len(usages) > 0 { // application
					page := g.page
					if len(usePages) > 0 && usePages[0] != 0 {
						page = usePages[0]
					}
					app = uint32(page)<<16 | uint32(usages[0])
				}
				apps = append(apps, app)
			case 0xC: // End Collection
				if len(apps) > 0 {
					apps = apps[:len(apps)-1]
				}
			}
			if tag == 0x8 || tag == 0x9 || tag == 0xB || tag == 0xA || tag == 0xC {
				clearLocals()
			}
		case 1: // global
			switch tag {
			case 0x0:
				g.page = uint16(u)
			case 0x1:
				g.logMin = s
			case 0x7:
				g.reportSize = int(u)
			case 0x8:
				g.reportID = int(u)
				layout.UsesIDs = true
			case 0x9:
				g.count = int(u)
			case 0xA:
				stack = append(stack, g)
			case 0xB:
				if len(stack) > 0 {
					g = stack[len(stack)-1]
					stack = stack[:len(stack)-1]
				}
			}
		case 2: // local
			switch tag {
			case 0x0:
				usages = append(usages, uint16(u))
				if size == 4 {
					usePages = append(usePages, uint16(u>>16))
				} else {
					usePages = append(usePages, 0)
				}
			case 0x1:
				uMin, haveMin = u, true
			case 0x2:
				uMax, haveMax = u, true
			}
		}
	}

	for _, f := range layout.Fields {
		if f.Page == pageKeyboard {
			layout.isKeyboard = true
		}
		if f.AppUsage == pageGenericDesktop<<16|usageMouse && f.Page == pageButton {
			layout.isMouse = true
		}
	}
	return layout
}

// fieldValue extracts value i of field f from report data (without the
// report ID byte). ok is false if the report is too short.
func fieldValue(data []byte, f *hidField, i int) (v int32, ok bool) {
	start := f.BitOffset + i*f.Size
	if f.Size > 32 || (start+f.Size+7)/8 > len(data) {
		return 0, false
	}
	var raw uint32
	for b := 0; b < f.Size; b++ {
		bit := start + b
		if data[bit/8]&(1<<(bit%8)) != 0 {
			raw |= 1 << b
		}
	}
	if f.LogMin < 0 && f.Size < 32 && raw&(1<<(f.Size-1)) != 0 {
		raw |= ^uint32(0) << f.Size // sign-extend
	}
	return int32(raw), true
}

// splitReport returns a report's ID and data, for a device's layout.
func splitReport(l *hidLayout, report []byte) (id int, data []byte) {
	if l.UsesIDs {
		if len(report) == 0 {
			return 0, nil
		}
		return int(report[0]), report[1:]
	}
	return 0, report
}
