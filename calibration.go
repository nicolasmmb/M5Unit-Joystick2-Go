package joystick2

import "time"

// Calibration is the ADC mapping stored in the unit's flash. The STM32 applies
// it to the raw ADC values before exposing Position.
type Calibration struct {
	XNegMin, XNegMax, XPosMin, XPosMax uint16
	YNegMin, YNegMax, YPosMin, YPosMax uint16
}

// AxisMapping is one axis of the calibration.
type AxisMapping struct {
	NegMin, NegMax, PosMin, PosMax uint16
}

// flashSettle is the time the STM32 needs after a mapping write to finish
// programming its flash (reads during that window may NACK).
const flashSettle = 20 * time.Millisecond

// Calibration reads the mapping currently stored in the unit.
func (d *Device) Calibration() (Calibration, error) {
	d.lock()
	defer d.unlock()
	if err := d.read(registerCalibration, d.rx[:16]); err != nil {
		return Calibration{}, err
	}
	get := func(i int) uint16 {
		return uint16(d.rx[i]) | uint16(d.rx[i+1])<<8
	}
	return Calibration{
		XNegMin: get(0), XNegMax: get(2), XPosMin: get(4), XPosMax: get(6),
		YNegMin: get(8), YNegMax: get(10), YPosMin: get(12), YPosMax: get(14),
	}, nil
}

// SetCalibration writes all eight mapping values. The unit persists them in
// flash, so avoid writing in a loop (flash endurance). Allow the unit to
// settle (see the package docs) before reading the mapping back.
func (d *Device) SetCalibration(c Calibration) error {
	d.lock()
	defer d.unlock()
	put := func(i int, v uint16) {
		d.tx[1+i] = uint8(v)
		d.tx[2+i] = uint8(v >> 8)
	}
	d.tx[0] = registerCalibration
	put(0, c.XNegMin)
	put(2, c.XNegMax)
	put(4, c.XPosMin)
	put(6, c.XPosMax)
	put(8, c.YNegMin)
	put(10, c.YNegMax)
	put(12, c.YPosMin)
	put(14, c.YPosMax)
	return d.bus.Tx(uint16(d.addr), d.tx[:17], nil)
}

// SetAxisMapping writes one axis of the mapping (the other axis is kept). The
// unit persists it in flash; allow it to settle before reading back.
func (d *Device) SetAxisMapping(axis Axis, m AxisMapping) error {
	var reg uint8
	switch axis {
	case AxisX:
		reg = registerCalibration
	case AxisY:
		reg = registerCalibration + 8
	default:
		return ErrAxis
	}
	d.lock()
	defer d.unlock()
	put := func(i int, v uint16) {
		d.tx[1+i] = uint8(v)
		d.tx[2+i] = uint8(v >> 8)
	}
	d.tx[0] = reg
	put(0, m.NegMin)
	put(2, m.NegMax)
	put(4, m.PosMin)
	put(6, m.PosMax)
	return d.bus.Tx(uint16(d.addr), d.tx[:9], nil)
}

// SetDeadzoneADC sets an ADC deadzone (0-32768) for both axes, like the
// official driver. Each axis write is persisted to flash; 100 ms is left
// between them for the STM32 to finish the flash write.
func (d *Device) SetDeadzoneADC(x, y uint16) error {
	if x > 32768 || y > 32768 {
		return ErrDeadzone
	}
	return d.setDeadzone(x/2, y/2)
}

// SetDeadzonePosition sets a deadzone in position units (0-4096), like the
// official driver. Each axis write is persisted to flash; 100 ms is left
// between them.
func (d *Device) SetDeadzonePosition(x, y uint16) error {
	if x > 4096 || y > 4096 {
		return ErrDeadzone
	}
	hx := uint16(uint32(32768) * uint32(x) / 4096)
	hy := uint16(uint32(32768) * uint32(y) / 4096)
	return d.setDeadzone(hx, hy)
}

func (d *Device) setDeadzone(halfX, halfY uint16) error {
	xm := deadzoneMapping(halfX)
	ym := deadzoneMapping(halfY)
	if err := d.SetAxisMapping(AxisX, xm); err != nil {
		return err
	}
	time.Sleep(axisWriteDelay)
	if err := d.SetAxisMapping(AxisY, ym); err != nil {
		return err
	}
	// The STM32 programs its flash after the write; give it time before a
	// read-back or the next I2C transaction.
	time.Sleep(flashSettle)
	return nil
}

func deadzoneMapping(half uint16) AxisMapping {
	posMin := uint32(32768) + uint32(half)
	if posMin > 65535 {
		posMin = 65535 // saturate at the 4096 position limit
	}
	return AxisMapping{
		NegMin: 0,
		NegMax: 32768 - half,
		PosMin: uint16(posMin),
		PosMax: 65535,
	}
}
