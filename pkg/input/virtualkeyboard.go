package input

import (
	"github.com/bendahl/uinput"
	"time"
)

// sleepTime defines the delay between key presses
const sleepTime = 40 * time.Millisecond

// VirtualKeyboard struct represents a virtual keyboard device
type VirtualKeyboard struct {
	Device uinput.Keyboard
}

// NewVirtualKeyboard creates and returns a new VirtualKeyboard instance
func NewVirtualKeyboard() (VirtualKeyboard, error) {
	var kb VirtualKeyboard

	vk, err := uinput.CreateKeyboard("/dev/uinput", []byte("mrext"))
	if err != nil {
		return kb, err
	}

	kb.Device = vk
	return kb, nil
}

// Close closes the virtual keyboard device
func (k *VirtualKeyboard) Close() {
	k.Device.Close()
}

// Press simulates pressing and releasing a key on the virtual keyboard
func (k *VirtualKeyboard) Press(key int) {
	k.Device.KeyDown(key)
	time.Sleep(sleepTime)
	k.Device.KeyUp(key)
}

// Combo simulates pressing and releasing a combination of keys
func (k *VirtualKeyboard) Combo(keys ...int) {
	for _, key := range keys {
		k.Device.KeyDown(key)
	}
	time.Sleep(sleepTime)
	for _, key := range keys {
		k.Device.KeyUp(key)
	}
}

// KeyDown simulates pressing a key down
func (k *VirtualKeyboard) KeyDown(key int) {
	k.Device.KeyDown(key)
}

// KeyUp simulates releasing a key
func (k *VirtualKeyboard) KeyUp(key int) {
	k.Device.KeyUp(key)
}

// VolumeUp simulates pressing the volume up key
func (k *VirtualKeyboard) VolumeUp() {
	k.Press(uinput.KeyVolumeup)
}

// VolumeDown simulates pressing the volume down key
func (k *VirtualKeyboard) VolumeDown() {
	k.Press(uinput.KeyVolumedown)
}

// VolumeMute simulates pressing the mute key
func (k *VirtualKeyboard) VolumeMute() {
	k.Press(uinput.KeyMute)
}

// Menu simulates pressing the "Menu" (Escape) key
func (k *VirtualKeyboard) Menu() {
	k.Press(uinput.KeyEsc)
}

// Back simulates pressing the backspace key
func (k *VirtualKeyboard) Back() {
	k.Press(uinput.KeyBackspace)
}

// Confirm simulates pressing the "Enter" key
func (k *VirtualKeyboard) Confirm() {
	k.Press(uinput.KeyEnter)
}

// Cancel simulates pressing the "Menu" key (Escape)
func (k *VirtualKeyboard) Cancel() {
	k.Menu()
}

// Up simulates pressing the "Up" arrow key
func (k *VirtualKeyboard) Up() {
	k.Press(uinput.KeyUp)
}

// Down simulates pressing the "Down" arrow key
func (k *VirtualKeyboard) Down() {
	k.Press(uinput.KeyDown)
}

// Left simulates pressing the "Left" arrow key
func (k *VirtualKeyboard) Left() {
	k.Press(uinput.KeyLeft)
}

// Right simulates pressing the "Right" arrow key
func (k *VirtualKeyboard) Right() {
	k.Press(uinput.KeyRight)
}

// Osd simulates pressing the "F12" key (for On-Screen Display)
func (k *VirtualKeyboard) Osd() {
	k.Press(uinput.KeyF12)
}

// CoreSelect simulates pressing the "LeftAlt + F12" keys for core selection
func (k *VirtualKeyboard) CoreSelect() {
	k.Combo(uinput.KeyLeftalt, uinput.KeyF12)
}

// Screenshot simulates pressing the "LeftAlt + ScrollLock" keys for taking a screenshot
func (k *VirtualKeyboard) Screenshot() {
	k.Combo(uinput.KeyLeftalt, uinput.KeyScrolllock)
}

// RawScreenshot simulates pressing the "LeftAlt + LeftShift + ScrollLock" keys for a raw screenshot
func (k *VirtualKeyboard) RawScreenshot() {
	k.Combo(uinput.KeyLeftalt, uinput.KeyLeftshift, uinput.KeyScrolllock)
}

// User simulates pressing the "LeftCtrl + LeftAlt + RightAlt" keys for user-defined action
func (k *VirtualKeyboard) User() {
	k.Combo(uinput.KeyLeftctrl, uinput.KeyLeftalt, uinput.KeyRightalt)
}

// Reset simulates pressing a combination of reset keys
func (k *VirtualKeyboard) Reset() {
	k.Combo(uinput.KeyLeftshift, uinput.KeyLeftctrl, uinput.KeyLeftalt, uinput.KeyRightalt)
}

// PairBluetooth simulates pressing the "F11" key for Bluetooth pairing
func (k *VirtualKeyboard) PairBluetooth() {
	k.Press(uinput.KeyF11)
}

// ChangeBackground simulates pressing the "F1" key for changing the background
func (k *VirtualKeyboard) ChangeBackground() {
	k.Press(uinput.KeyF1)
}

// ToggleCoreDates simulates pressing the "F2" key for toggling core dates
func (k *VirtualKeyboard) ToggleCoreDates() {
	k.Press(uinput.KeyF2)
}

// Console simulates pressing the "F9" key to open the console
func (k *VirtualKeyboard) Console() {
	k.Press(uinput.KeyF9)
}

// ExitConsole simulates pressing the "F12" key to exit the console
func (k *VirtualKeyboard) ExitConsole() {
	k.Press(uinput.KeyF12)
}

// ComputerOsd simulates pressing the "LeftMeta + F12" keys for a system OSD
func (k *VirtualKeyboard) ComputerOsd() {
	k.Combo(uinput.KeyLeftmeta, uinput.KeyF12)
}
