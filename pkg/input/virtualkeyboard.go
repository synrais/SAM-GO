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
