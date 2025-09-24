package input

import (
	"time"

	"github.com/bendahl/uinput"
)

// delay between key presses
const sleepTime = 40 * time.Millisecond

// VirtualKeyboard wraps a uinput.Keyboard device
type VirtualKeyboard struct {
	Device uinput.Keyboard
}

// NewVirtualKeyboard creates and returns a new VirtualKeyboard
func NewVirtualKeyboard() (*VirtualKeyboard, error) {
	vk, err := uinput.CreateKeyboard("/dev/uinput", []byte("SAM_Keyboard"))
	if err != nil {
		return nil, err
	}
	return &VirtualKeyboard{Device: vk}, nil
}

// Close releases the keyboard device
func (k *VirtualKeyboard) Close() error {
	return k.Device.Close()
}

// Press simulates a single key press
func (k *VirtualKeyboard) Press(key int) error {
	if err := k.Device.KeyDown(key); err != nil {
		return err
	}
	time.Sleep(sleepTime)
	if err := k.Device.KeyUp(key); err != nil {
		return err
	}
	return nil
}

// Combo simulates pressing and releasing a combination of keys
func (k *VirtualKeyboard) Combo(keys ...int) error {
	for _, key := range keys {
		if err := k.Device.KeyDown(key); err != nil {
			return err
		}
	}
	time.Sleep(sleepTime)
	for _, key := range keys {
		if err := k.Device.KeyUp(key); err != nil {
			return err
		}
	}
	return nil
}

// --- Convenience Wrappers ---

func (k *VirtualKeyboard) VolumeUp() error    { return k.Press(uinput.KeyVolumeup) }
func (k *VirtualKeyboard) VolumeDown() error  { return k.Press(uinput.KeyVolumedown) }
func (k *VirtualKeyboard) VolumeMute() error  { return k.Press(uinput.KeyMute) }
func (k *VirtualKeyboard) Menu() error        { return k.Press(uinput.KeyEsc) }
func (k *VirtualKeyboard) Back() error        { return k.Press(uinput.KeyBackspace) }
func (k *VirtualKeyboard) Confirm() error     { return k.Press(uinput.KeyEnter) }
func (k *VirtualKeyboard) Cancel() error      { return k.Menu() }
func (k *VirtualKeyboard) Up() error          { return k.Press(uinput.KeyUp) }
func (k *VirtualKeyboard) Down() error        { return k.Press(uinput.KeyDown) }
func (k *VirtualKeyboard) Left() error        { return k.Press(uinput.KeyLeft) }
func (k *VirtualKeyboard) Right() error       { return k.Press(uinput.KeyRight) }
func (k *VirtualKeyboard) Osd() error         { return k.Press(uinput.KeyF12) }
func (k *VirtualKeyboard) CoreSelect() error  { return k.Combo(uinput.KeyLeftalt, uinput.KeyF12) }
func (k *VirtualKeyboard) Screenshot() error  { return k.Combo(uinput.KeyLeftalt, uinput.KeyScrolllock) }
func (k *VirtualKeyboard) RawScreenshot() error {
	return k.Combo(uinput.KeyLeftalt, uinput.KeyLeftshift, uinput.KeyScrolllock)
}
func (k *VirtualKeyboard) User() error {
	return k.Combo(uinput.KeyLeftctrl, uinput.KeyLeftalt, uinput.KeyRightalt)
}
func (k *VirtualKeyboard) Reset() error {
	return k.Combo(uinput.KeyLeftshift, uinput.KeyLeftctrl, uinput.KeyLeftalt, uinput.KeyRightalt)
}
func (k *VirtualKeyboard) PairBluetooth() error { return k.Press(uinput.KeyF11) }
func (k *VirtualKeyboard) ChangeBackground() error {
	return k.Press(uinput.KeyF1)
}
func (k *VirtualKeyboard) ToggleCoreDates() error {
	return k.Press(uinput.KeyF2)
}
func (k *VirtualKeyboard) Console() error     { return k.Press(uinput.KeyF9) }
func (k *VirtualKeyboard) ExitConsole() error { return k.Press(uinput.KeyF12) }
func (k *VirtualKeyboard) ComputerOsd() error {
	return k.Combo(uinput.KeyLeftmeta, uinput.KeyF12)
}
