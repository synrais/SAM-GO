package input

import (
	"errors"
	"sync"
	"time"

	"github.com/bendahl/uinput"
)

// delay between key presses
const sleepTime = 40 * time.Millisecond

// VirtualKeyboard wraps a uinput.Keyboard device
type VirtualKeyboard struct {
	Device uinput.Keyboard
}

var (
	keyboardInstance *VirtualKeyboard
	keyboardMu       sync.Mutex
)

// NewVirtualKeyboard creates and returns the singleton VirtualKeyboard
func NewVirtualKeyboard() (*VirtualKeyboard, error) {
	keyboardMu.Lock()
	defer keyboardMu.Unlock()

	if keyboardInstance != nil {
		return keyboardInstance, nil
	}

	vk, err := uinput.CreateKeyboard("/dev/uinput", []byte("SAM_Keyboard"))
	if err != nil {
		return nil, err
	}

	keyboardInstance = &VirtualKeyboard{Device: vk}
	return keyboardInstance, nil
}

// Close releases the keyboard device and clears the singleton
func (k *VirtualKeyboard) Close() error {
	keyboardMu.Lock()
	defer keyboardMu.Unlock()

	if keyboardInstance == nil {
		return errors.New("no keyboard to close")
	}

	err := keyboardInstance.Device.Close()
	keyboardInstance = nil
	return err
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
