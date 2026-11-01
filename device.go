// Package joystick2 drives the M5Stack Unit Joystick2 (SKU U024-V2), a
// Hall-effect 3-axis joystick with an onboard STM32G030 and a WS2812C LED.
//
// The driver is pure Go, uses fixed buffers and performs no heap allocation per
// operation. Register access goes through a minimal I2C interface so the same
// code runs on TinyGo targets and in host tests.
package joystick2

import (
	"errors"
	"sync"
	"time"
)

// I2C is the subset of tinygo.org/x/drivers.I2C used by the unit.
// machine.I2C satisfies it.
type I2C interface {
	// Tx writes w to addr, then reads len(r) bytes into r.
	Tx(addr uint16, w, r []byte) error
}

const (
	// DefaultAddress is the 7-bit I2C address of the Unit Joystick2.
	DefaultAddress = 0x63

	registerRaw         = 0x00 // X/Y 16-bit ADC, u16 LE
	registerRaw8        = 0x10 // X/Y 8-bit ADC
	registerButton      = 0x20 // 1 = released, 0 = pressed
	registerLED         = 0x30 // u32 LE: B, G, R, A
	registerCalibration = 0x40 // 8 x u16 LE
	registerPosition    = 0x50 // X/Y int16 LE, +-4095
	registerPosition8   = 0x60 // X/Y int8
	registerBootloader  = 0xFC
	registerFirmware    = 0xFE
	registerAddress     = 0xFF

	// axisWriteDelay matches the official driver: the STM32 persists the
	// calibration to flash between the X and Y writes.
	axisWriteDelay = 100 * time.Millisecond
	// addressWriteDelay gives the firmware time to persist the new address
	// and restart its I2C peripheral.
	addressWriteDelay = 10 * time.Millisecond
)

// Errors returned by the driver.
var (
	ErrAddress    = errors.New("joystick2: address must be 0x08-0x77")
	ErrBrightness = errors.New("joystick2: brightness must be 0-100")
	ErrDeadzone   = errors.New("joystick2: deadzone out of range")
	ErrAxis       = errors.New("joystick2: invalid axis")
)

// Axis selects one joystick axis for mapping.
type Axis uint8

const (
	AxisX Axis = iota
	AxisY
)

// Color is an RGB LED color.
type Color struct {
	R, G, B uint8
}

// Orientation is applied by the driver to the values read from the unit.
type Orientation struct {
	InvertX bool
	InvertY bool
	Swap    bool
}

// Sample is one UI reading: position plus the Z-axis button.
type Sample struct {
	X, Y    int16
	Pressed bool
}

// Device is a Unit Joystick2. It is safe for concurrent use: every public
// method serializes on an internal mutex. Each operation is one or two I2C
// transactions, so there are no long polling loops to cancel.
type Device struct {
	mu sync.Mutex

	bus  I2C
	addr uint8

	orientation Orientation

	// Cached LED color (unscaled) and brightness, like the official driver.
	color      Color
	brightness uint8
	cacheValid bool

	// Scratch buffers: reusing them keeps every transaction allocation-free.
	tx [17]byte
	rx [16]byte
}

// New returns a Device on bus with the default address.
func New(bus I2C) *Device {
	return &Device{
		bus:        bus,
		addr:       DefaultAddress,
		brightness: 100,
	}
}

// NewAt is New with a non-default I2C address.
func NewAt(bus I2C, address uint8) *Device {
	d := New(bus)
	d.addr = address & 0x7F
	return d
}

func (d *Device) lock()   { d.mu.Lock() }
func (d *Device) unlock() { d.mu.Unlock() }

// Address returns the configured 7-bit I2C address.
func (d *Device) Address() uint8 {
	d.lock()
	a := d.addr
	d.unlock()
	return a
}

// Present reports whether the unit answers on the bus.
func (d *Device) Present() bool {
	d.lock()
	defer d.unlock()
	_, err := d.readU8(registerFirmware)
	return err == nil
}

// ReadAddress reads the address register from the unit (which may differ from
// the driver's cached Address after an external change).
func (d *Device) ReadAddress() (uint8, error) {
	d.lock()
	defer d.unlock()
	return d.readU8(registerAddress)
}

// FirmwareVersion reads the firmware version register.
func (d *Device) FirmwareVersion() (uint8, error) {
	d.lock()
	defer d.unlock()
	return d.readU8(registerFirmware)
}

// BootloaderVersion reads the bootloader version register.
func (d *Device) BootloaderVersion() (uint8, error) {
	d.lock()
	defer d.unlock()
	return d.readU8(registerBootloader)
}

// SetAddress changes the I2C address and persists it in the unit's flash. The
// new address stays in effect until changed again.
func (d *Device) SetAddress(address uint8) error {
	if address < 0x08 || address > 0x77 {
		return ErrAddress
	}
	d.lock()
	defer d.unlock()
	if err := d.write(registerAddress, address&0x7F); err != nil {
		return err
	}
	d.addr = address
	time.Sleep(addressWriteDelay)
	return nil
}

// SetOrientation configures the inversion/swap applied to readings.
func (d *Device) SetOrientation(o Orientation) {
	d.lock()
	d.orientation = o
	d.unlock()
}

// Orientation returns the current inversion/swap configuration.
func (d *Device) Orientation() Orientation {
	d.lock()
	o := d.orientation
	d.unlock()
	return o
}

// Raw reads the 16-bit ADC values (0-65535).
func (d *Device) Raw() (x, y uint16, err error) {
	d.lock()
	defer d.unlock()
	if err := d.read(registerRaw, d.rx[:4]); err != nil {
		return 0, 0, err
	}
	x = uint16(d.rx[0]) | uint16(d.rx[1])<<8
	y = uint16(d.rx[2]) | uint16(d.rx[3])<<8
	x, y = d.orientRaw(x, y)
	return x, y, nil
}

// RawX reads only the X axis of the 16-bit ADC (orientation applied).
func (d *Device) RawX() (uint16, error) {
	x, _, err := d.Raw()
	return x, err
}

// RawY reads only the Y axis of the 16-bit ADC (orientation applied).
func (d *Device) RawY() (uint16, error) {
	_, y, err := d.Raw()
	return y, err
}

// Raw8 reads the 8-bit ADC values (0-255).
func (d *Device) Raw8() (x, y uint8, err error) {
	d.lock()
	defer d.unlock()
	if err := d.read(registerRaw8, d.rx[:2]); err != nil {
		return 0, 0, err
	}
	x, y = d.rx[0], d.rx[1]
	x, y = d.orientRaw8(x, y)
	return x, y, nil
}

// Button reports whether the Z-axis button is pressed.
func (d *Device) Button() (bool, error) {
	d.lock()
	defer d.unlock()
	v, err := d.readU8(registerButton)
	if err != nil {
		return false, err
	}
	return v == 0, nil
}

// Position reads the mapped position (+-4095).
func (d *Device) Position() (x, y int16, err error) {
	d.lock()
	defer d.unlock()
	return d.positionLocked()
}

// PositionX reads only the X axis of the mapped position (orientation applied).
func (d *Device) PositionX() (int16, error) {
	x, _, err := d.Position()
	return x, err
}

// PositionY reads only the Y axis of the mapped position (orientation applied).
func (d *Device) PositionY() (int16, error) {
	_, y, err := d.Position()
	return y, err
}

// Position8 reads the 8-bit mapped position (+-127).
func (d *Device) Position8() (x, y int8, err error) {
	d.lock()
	defer d.unlock()
	if err := d.read(registerPosition8, d.rx[:2]); err != nil {
		return 0, 0, err
	}
	x, y = int8(d.rx[0]), int8(d.rx[1])
	x, y = d.orientPos8(x, y)
	return x, y, nil
}

// Sample reads position and button in two transactions (the UI fast path).
func (d *Device) Sample() (Sample, error) {
	d.lock()
	defer d.unlock()
	x, y, err := d.positionLocked()
	if err != nil {
		return Sample{}, err
	}
	v, err := d.readU8(registerButton)
	if err != nil {
		return Sample{}, err
	}
	return Sample{X: x, Y: y, Pressed: v == 0}, nil
}

func (d *Device) positionLocked() (x, y int16, err error) {
	if err := d.read(registerPosition, d.rx[:4]); err != nil {
		return 0, 0, err
	}
	x = int16(uint16(d.rx[0]) | uint16(d.rx[1])<<8)
	y = int16(uint16(d.rx[2]) | uint16(d.rx[3])<<8)
	x, y = d.orientPos(x, y)
	return x, y, nil
}

func (d *Device) orientRaw(x, y uint16) (uint16, uint16) {
	if d.orientation.InvertX {
		x = 65535 - x
	}
	if d.orientation.InvertY {
		y = 65535 - y
	}
	if d.orientation.Swap {
		x, y = y, x
	}
	return x, y
}

func (d *Device) orientRaw8(x, y uint8) (uint8, uint8) {
	if d.orientation.InvertX {
		x = 255 - x
	}
	if d.orientation.InvertY {
		y = 255 - y
	}
	if d.orientation.Swap {
		x, y = y, x
	}
	return x, y
}

func (d *Device) orientPos(x, y int16) (int16, int16) {
	if d.orientation.InvertX {
		x = -x
	}
	if d.orientation.InvertY {
		y = -y
	}
	if d.orientation.Swap {
		x, y = y, x
	}
	return x, y
}

func (d *Device) orientPos8(x, y int8) (int8, int8) {
	if d.orientation.InvertX {
		x = -x
	}
	if d.orientation.InvertY {
		y = -y
	}
	if d.orientation.Swap {
		x, y = y, x
	}
	return x, y
}

// --- register helpers ---

func (d *Device) write(reg, val uint8) error {
	d.tx[0] = reg
	d.tx[1] = val
	return d.bus.Tx(uint16(d.addr), d.tx[:2], nil)
}

func (d *Device) read(reg uint8, buf []byte) error {
	d.tx[0] = reg
	return d.bus.Tx(uint16(d.addr), d.tx[:1], buf)
}

func (d *Device) readU8(reg uint8) (uint8, error) {
	d.tx[0] = reg
	if err := d.bus.Tx(uint16(d.addr), d.tx[:1], d.rx[:1]); err != nil {
		return 0, err
	}
	return d.rx[0], nil
}

func (d *Device) readU16(reg uint8) (uint16, error) {
	d.tx[0] = reg
	if err := d.bus.Tx(uint16(d.addr), d.tx[:1], d.rx[:2]); err != nil {
		return 0, err
	}
	return uint16(d.rx[0]) | uint16(d.rx[1])<<8, nil
}
