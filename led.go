package joystick2

// SetLED sets the RGB LED color, scaled by the current brightness. The color
// is cached so changing the brightness later does not need to read the unit.
func (d *Device) SetLED(c Color) error {
	d.lock()
	defer d.unlock()
	d.color = c
	return d.writeLEDLocked()
}

// SetLEDHex sets the color from a 0xRRGGBB value (the UIFlow fill_color
// input). The brightness is applied once.
func (d *Device) SetLEDHex(rgb uint32) error {
	return d.SetLED(Color{R: uint8(rgb >> 16), G: uint8(rgb >> 8), B: uint8(rgb)})
}

// LEDHex returns the color currently set on the unit as 0xRRGGBB.
func (d *Device) LEDHex() (uint32, error) {
	c, err := d.LED()
	if err != nil {
		return 0, err
	}
	return uint32(c.R)<<16 | uint32(c.G)<<8 | uint32(c.B), nil
}

// SetBrightness sets the LED brightness (0-100) and rewrites the cached color.
// Unlike the UIFlow helper, the brightness is applied exactly once.
func (d *Device) SetBrightness(percent uint8) error {
	if percent > 100 {
		return ErrBrightness
	}
	d.lock()
	defer d.unlock()
	d.brightness = percent
	if !d.cacheValid {
		return nil
	}
	return d.writeLEDLocked()
}

// Brightness returns the configured LED brightness (0-100).
func (d *Device) Brightness() uint8 {
	d.lock()
	b := d.brightness
	d.unlock()
	return b
}

// LED reads the color currently set on the unit (already scaled by the
// brightness that was active when it was written).
func (d *Device) LED() (Color, error) {
	d.lock()
	defer d.unlock()
	if err := d.read(registerLED, d.rx[:4]); err != nil {
		return Color{}, err
	}
	return Color{R: d.rx[2], G: d.rx[1], B: d.rx[0]}, nil
}

func (d *Device) writeLEDLocked() error {
	d.tx[0] = registerLED
	d.tx[1] = scaleByte(d.color.B, d.brightness)
	d.tx[2] = scaleByte(d.color.G, d.brightness)
	d.tx[3] = scaleByte(d.color.R, d.brightness)
	if err := d.bus.Tx(uint16(d.addr), d.tx[:4], nil); err != nil {
		return err
	}
	d.cacheValid = true
	return nil
}

func scaleByte(v, percent uint8) uint8 {
	return uint8(uint16(v) * uint16(percent) / 100)
}
