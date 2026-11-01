//go:build esp32

// Example: read the M5Stack Unit Joystick2 over Grove I2C.
package main

import (
	"time"

	"machine"

	joystick2 "github.com/nicolasmmb/M5Unit-Joystick2-Go"
)

func main() {
	time.Sleep(time.Second)

	i2c := machine.I2C1
	if err := i2c.Configure(machine.I2CConfig{
		SDA:       machine.GPIO32,
		SCL:       machine.GPIO33,
		Frequency: 100 * machine.KHz,
	}); err != nil {
		println("joystick2: i2c:", err.Error())
		return
	}

	d := joystick2.New(i2c)
	if !d.Present() {
		println("joystick2: not found at", d.Address())
		return
	}
	_ = d.SetLED(joystick2.Color{R: 0, G: 64, B: 64})
	fw, _ := d.FirmwareVersion()
	println("joystick2: firmware", fw, "address", d.Address())

	for {
		s, err := d.Sample()
		if err != nil {
			println("joystick2:", err.Error())
			time.Sleep(100 * time.Millisecond)
			continue
		}
		println("joystick2: x", s.X, "y", s.Y, "pressed", s.Pressed)
		time.Sleep(50 * time.Millisecond)
	}
}
