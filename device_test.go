package joystick2

import (
	"errors"
	"testing"
	"unsafe"
)

// fakeSensor emulates the Unit Joystick2 register map, including the flash that
// holds the calibration and the I2C address.
type fakeSensor struct {
	regs [256]byte
	addr uint16
	tx   int
	fail bool
}

func newFakeSensor() *fakeSensor {
	f := &fakeSensor{addr: DefaultAddress}
	f.regs[registerFirmware] = 1
	f.regs[registerBootloader] = 2
	f.regs[registerAddress] = DefaultAddress
	f.regs[registerButton] = 1 // released
	// Defaults from the official firmware.
	cal := Calibration{
		XNegMin: 130, XNegMax: 33330, XPosMin: 34000, XPosMax: 65440,
		YNegMin: 130, YNegMax: 33330, YPosMin: 34000, YPosMax: 65440,
	}
	f.setCal(cal)
	f.setRaw(33000, 32000)
	f.setRaw8(128, 120)
	f.setPosition(100, -200)
	f.setPosition8(3, -4)
	f.setLED(0, 0, 0)
	return f
}

func (f *fakeSensor) setU16(reg uint8, v uint16) {
	f.regs[reg] = byte(v)
	f.regs[reg+1] = byte(v >> 8)
}

func (f *fakeSensor) setI16(reg uint8, v int16) { f.setU16(reg, uint16(v)) }

func (f *fakeSensor) setRaw(x, y uint16) {
	f.setU16(registerRaw, x)
	f.setU16(registerRaw+2, y)
}

func (f *fakeSensor) setRaw8(x, y uint8) {
	f.regs[registerRaw8] = x
	f.regs[registerRaw8+1] = y
}

func (f *fakeSensor) setPosition(x, y int16) {
	f.setI16(registerPosition, x)
	f.setI16(registerPosition+2, y)
}

func (f *fakeSensor) setPosition8(x, y int8) {
	f.regs[registerPosition8] = byte(x)
	f.regs[registerPosition8+1] = byte(y)
}

func (f *fakeSensor) setCal(c Calibration) {
	vals := []uint16{c.XNegMin, c.XNegMax, c.XPosMin, c.XPosMax,
		c.YNegMin, c.YNegMax, c.YPosMin, c.YPosMax}
	for i, v := range vals {
		f.setU16(registerCalibration+uint8(i*2), v)
	}
}

func (f *fakeSensor) cal() Calibration {
	get := func(off uint8) uint16 {
		return uint16(f.regs[registerCalibration+off]) | uint16(f.regs[registerCalibration+off+1])<<8
	}
	return Calibration{
		XNegMin: get(0), XNegMax: get(2), XPosMin: get(4), XPosMax: get(6),
		YNegMin: get(8), YNegMax: get(10), YPosMin: get(12), YPosMax: get(14),
	}
}

func (f *fakeSensor) setLED(r, g, b uint8) {
	f.regs[registerLED] = b
	f.regs[registerLED+1] = g
	f.regs[registerLED+2] = r
	f.regs[registerLED+3] = 0
}

func (f *fakeSensor) Tx(addr uint16, w, r []byte) error {
	if f.fail {
		return errors.New("i2c: no ack")
	}
	if addr != f.addr {
		return errors.New("i2c: no ack")
	}
	f.tx++
	if len(w) == 0 {
		return nil
	}
	reg := w[0]
	if len(r) > 0 {
		for i := range r {
			r[i] = f.regs[reg+uint8(i)]
		}
		return nil
	}
	for i := 1; i < len(w); i++ {
		f.regs[reg+uint8(i)-1] = w[i]
	}
	if reg == registerAddress && len(w) == 2 {
		f.addr = uint16(w[1] & 0x7F)
	}
	return nil
}

func newTestDevice(tb testing.TB) (*Device, *fakeSensor) {
	tb.Helper()
	f := newFakeSensor()
	d := New(f)
	return d, f
}

func TestVersions(t *testing.T) {
	d, _ := newTestDevice(t)
	if !d.Present() {
		t.Fatal("Present = false")
	}
	if v, err := d.FirmwareVersion(); err != nil || v != 1 {
		t.Fatalf("FirmwareVersion = %d, %v", v, err)
	}
	if v, err := d.BootloaderVersion(); err != nil || v != 2 {
		t.Fatalf("BootloaderVersion = %d, %v", v, err)
	}
	if v := d.Address(); v != DefaultAddress {
		t.Fatalf("Address = %#x", v)
	}
}

func TestRaw(t *testing.T) {
	d, f := newTestDevice(t)
	f.setRaw(1000, 65000)
	x, y, err := d.Raw()
	if err != nil || x != 1000 || y != 65000 {
		t.Fatalf("Raw = %d, %d, %v", x, y, err)
	}
}

func TestAxisGetters(t *testing.T) {
	d, f := newTestDevice(t)
	f.setRaw(111, 222)
	f.setPosition(-333, 444)
	if x, err := d.RawX(); err != nil || x != 111 {
		t.Fatalf("RawX = %d, %v", x, err)
	}
	if y, err := d.RawY(); err != nil || y != 222 {
		t.Fatalf("RawY = %d, %v", y, err)
	}
	if x, err := d.PositionX(); err != nil || x != -333 {
		t.Fatalf("PositionX = %d, %v", x, err)
	}
	if y, err := d.PositionY(); err != nil || y != 444 {
		t.Fatalf("PositionY = %d, %v", y, err)
	}
	d.SetOrientation(Orientation{Swap: true})
	if x, err := d.RawX(); err != nil || x != 222 {
		t.Fatalf("RawX swapped = %d, %v", x, err)
	}
	if y, err := d.PositionY(); err != nil || y != -333 {
		t.Fatalf("PositionY swapped = %d, %v", y, err)
	}
}

func TestReadAddress(t *testing.T) {
	d, f := newTestDevice(t)
	if a, err := d.ReadAddress(); err != nil || a != DefaultAddress {
		t.Fatalf("ReadAddress = %#x, %v", a, err)
	}
	f.regs[registerAddress] = 0x2B
	if a, err := d.ReadAddress(); err != nil || a != 0x2B {
		t.Fatalf("ReadAddress after change = %#x, %v", a, err)
	}
}

func TestRaw8(t *testing.T) {
	d, f := newTestDevice(t)
	f.setRaw8(10, 250)
	x, y, err := d.Raw8()
	if err != nil || x != 10 || y != 250 {
		t.Fatalf("Raw8 = %d, %d, %v", x, y, err)
	}
}

func TestButton(t *testing.T) {
	d, f := newTestDevice(t)
	f.regs[registerButton] = 1
	if pressed, err := d.Button(); err != nil || pressed {
		t.Fatalf("Button = %v, %v; want released", pressed, err)
	}
	f.regs[registerButton] = 0
	if pressed, err := d.Button(); err != nil || !pressed {
		t.Fatalf("Button = %v, %v; want pressed", pressed, err)
	}
}

func TestPosition(t *testing.T) {
	d, f := newTestDevice(t)
	f.setPosition(-4095, 4095)
	x, y, err := d.Position()
	if err != nil || x != -4095 || y != 4095 {
		t.Fatalf("Position = %d, %d, %v", x, y, err)
	}
	f.setPosition8(-127, 127)
	x8, y8, err := d.Position8()
	if err != nil || x8 != -127 || y8 != 127 {
		t.Fatalf("Position8 = %d, %d, %v", x8, y8, err)
	}
}

func TestSample(t *testing.T) {
	d, f := newTestDevice(t)
	f.setPosition(-300, 700)
	f.regs[registerButton] = 0
	s, err := d.Sample()
	if err != nil {
		t.Fatal(err)
	}
	if s.X != -300 || s.Y != 700 || !s.Pressed {
		t.Fatalf("Sample = %+v", s)
	}
}

func TestOrientation(t *testing.T) {
	d, f := newTestDevice(t)
	d.SetOrientation(Orientation{InvertX: true, Swap: true})
	f.setPosition(100, -200)
	x, y, err := d.Position()
	if err != nil || x != -200 || y != -100 {
		t.Fatalf("Position swap/invert = %d, %d, %v", x, y, err)
	}
	f.setRaw(1000, 2000)
	rx, ry, err := d.Raw()
	if err != nil || rx != 2000 || ry != 64535 {
		t.Fatalf("Raw swap/invert = %d, %d, %v", rx, ry, err)
	}
}

func TestLED(t *testing.T) {
	d, f := newTestDevice(t)
	if err := d.SetLED(Color{R: 200, G: 100, B: 50}); err != nil {
		t.Fatal(err)
	}
	if f.regs[registerLED] != 50 || f.regs[registerLED+1] != 100 || f.regs[registerLED+2] != 200 {
		t.Fatalf("LED bytes = %v", f.regs[registerLED:registerLED+4])
	}
	c, err := d.LED()
	if err != nil || c != (Color{R: 200, G: 100, B: 50}) {
		t.Fatalf("LED = %+v, %v", c, err)
	}

	if err := d.SetLEDHex(0x112233); err != nil {
		t.Fatal(err)
	}
	if c, err := d.LED(); err != nil || c != (Color{R: 0x11, G: 0x22, B: 0x33}) {
		t.Fatalf("SetLEDHex = %+v, %v", c, err)
	}
	if v, err := d.LEDHex(); err != nil || v != 0x112233 {
		t.Fatalf("LEDHex = %#x, %v", v, err)
	}
	if err := d.SetLED(Color{R: 200, G: 100, B: 50}); err != nil {
		t.Fatal(err)
	}

	if err := d.SetBrightness(50); err != nil {
		t.Fatal(err)
	}
	if f.regs[registerLED] != 25 || f.regs[registerLED+1] != 50 || f.regs[registerLED+2] != 100 {
		t.Fatalf("dimmed LED bytes = %v", f.regs[registerLED:registerLED+4])
	}
	if err := d.SetBrightness(0); err != nil {
		t.Fatal(err)
	}
	if f.regs[registerLED] != 0 || f.regs[registerLED+1] != 0 || f.regs[registerLED+2] != 0 {
		t.Fatalf("off LED bytes = %v", f.regs[registerLED:registerLED+4])
	}
	if err := d.SetBrightness(101); !errors.Is(err, ErrBrightness) {
		t.Fatalf("SetBrightness(101) = %v", err)
	}
}

func TestCalibration(t *testing.T) {
	d, f := newTestDevice(t)
	got, err := d.Calibration()
	if err != nil {
		t.Fatal(err)
	}
	if got != f.cal() || got.XNegMin != 130 || got.YPosMax != 65440 {
		t.Fatalf("Calibration = %+v", got)
	}

	want := Calibration{
		XNegMin: 100, XNegMax: 30000, XPosMin: 35000, XPosMax: 65000,
		YNegMin: 200, YNegMax: 31000, YPosMin: 36000, YPosMax: 65000,
	}
	if err := d.SetCalibration(want); err != nil {
		t.Fatal(err)
	}
	if f.cal() != want {
		t.Fatalf("stored calibration = %+v", f.cal())
	}

	xm := AxisMapping{NegMin: 1, NegMax: 2, PosMin: 3, PosMax: 4}
	if err := d.SetAxisMapping(AxisX, xm); err != nil {
		t.Fatal(err)
	}
	if got := f.cal(); got.XNegMin != 1 || got.XPosMax != 4 || got.YNegMin != 200 {
		t.Fatalf("axis mapping = %+v", got)
	}
	if err := d.SetAxisMapping(Axis(9), xm); !errors.Is(err, ErrAxis) {
		t.Fatalf("SetAxisMapping(9) = %v", err)
	}
}

func TestDeadzone(t *testing.T) {
	d, f := newTestDevice(t)
	if err := d.SetDeadzoneADC(200, 100); err != nil {
		t.Fatal(err)
	}
	cal := f.cal()
	if cal.XNegMax != 32768-100 || cal.XPosMin != 32768+100 {
		t.Fatalf("X deadzone = %+v", cal)
	}
	if cal.YNegMax != 32768-50 || cal.YPosMin != 32768+50 {
		t.Fatalf("Y deadzone = %+v", cal)
	}
	if err := d.SetDeadzoneADC(32769, 0); !errors.Is(err, ErrDeadzone) {
		t.Fatalf("SetDeadzoneADC(32769) = %v", err)
	}
	if err := d.SetDeadzonePosition(4096, 0); err != nil {
		t.Fatal(err)
	}
	if cal := f.cal(); cal.XNegMax != 0 || cal.XPosMin != 65535 {
		t.Fatalf("position deadzone = %+v", cal)
	}
	if err := d.SetDeadzonePosition(4097, 0); !errors.Is(err, ErrDeadzone) {
		t.Fatalf("SetDeadzonePosition(4097) = %v", err)
	}
}

func TestSetAddress(t *testing.T) {
	d, f := newTestDevice(t)
	for _, bad := range []uint8{0x00, 0x07, 0x78, 0xFF} {
		if err := d.SetAddress(bad); !errors.Is(err, ErrAddress) {
			t.Fatalf("SetAddress(%#x) = %v", bad, err)
		}
	}
	if err := d.SetAddress(0x2A); err != nil {
		t.Fatal(err)
	}
	if d.Address() != 0x2A || f.addr != 0x2A {
		t.Fatalf("address = %#x / fake %#x", d.Address(), f.addr)
	}
	if v, err := d.FirmwareVersion(); err != nil || v != 1 {
		t.Fatalf("FirmwareVersion at new address = %d, %v", v, err)
	}
	var b [1]byte
	if err := f.Tx(DefaultAddress, []byte{registerFirmware}, b[:]); err == nil {
		t.Fatal("old address still responds")
	}
}

func TestNotConnected(t *testing.T) {
	f := newFakeSensor()
	f.fail = true
	d := New(f)
	if d.Present() {
		t.Fatal("Present = true with no ACK")
	}
	if _, err := d.Sample(); err == nil {
		t.Fatal("Sample succeeded with no ACK")
	}
}

func TestTxCounts(t *testing.T) {
	d, _ := newTestDevice(t)
	check := func(name string, want int, fn func() error) {
		t.Helper()
		before := d.bus.(*fakeSensor).tx
		if err := fn(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := d.bus.(*fakeSensor).tx - before; got != want {
			t.Fatalf("%s used %d transactions, want %d", name, got, want)
		}
	}
	check("Position", 1, func() error { _, _, err := d.Position(); return err })
	check("Raw", 1, func() error { _, _, err := d.Raw(); return err })
	check("Sample", 2, func() error { _, err := d.Sample(); return err })
	check("Button", 1, func() error { _, err := d.Button(); return err })
	check("SetLED", 1, func() error { return d.SetLED(Color{R: 10, G: 20, B: 30}) })
	check("Calibration", 1, func() error { _, err := d.Calibration(); return err })
	check("LED", 1, func() error { _, err := d.LED(); return err })
}

func TestNoAlloc(t *testing.T) {
	d, f := newTestDevice(t)
	checks := []struct {
		name string
		fn   func()
	}{
		{"Raw", func() { _, _, _ = d.Raw() }},
		{"Raw8", func() { _, _, _ = d.Raw8() }},
		{"Button", func() { _, _ = d.Button() }},
		{"Position", func() { _, _, _ = d.Position() }},
		{"Position8", func() { _, _, _ = d.Position8() }},
		{"Sample", func() { _, _ = d.Sample() }},
		{"SetLED", func() { _ = d.SetLED(Color{R: 10, G: 20, B: 30}) }},
		{"SetBrightness", func() { _ = d.SetBrightness(50) }},
		{"LED", func() { _, _ = d.LED() }},
		{"Calibration", func() { _, _ = d.Calibration() }},
		{"SetAxisMapping", func() { _ = d.SetAxisMapping(AxisX, AxisMapping{1, 2, 3, 4}) }},
		{"FirmwareVersion", func() { _, _ = d.FirmwareVersion() }},
		{"RawX", func() { _, _ = d.RawX() }},
		{"PositionX", func() { _, _ = d.PositionX() }},
		{"ReadAddress", func() { _, _ = d.ReadAddress() }},
		{"SetLEDHex", func() { _ = d.SetLEDHex(0x204060) }},
		{"LEDHex", func() { _, _ = d.LEDHex() }},
	}
	for _, c := range checks {
		if allocs := testing.AllocsPerRun(50, c.fn); allocs != 0 {
			t.Errorf("%s allocates %v per call", c.name, allocs)
		}
	}
	_ = f
}

func TestFootprint(t *testing.T) {
	t.Logf("Color=%d B  Sample=%d B  Calibration=%d B  Device=%d B",
		unsafe.Sizeof(Color{}), unsafe.Sizeof(Sample{}), unsafe.Sizeof(Calibration{}), unsafe.Sizeof(Device{}))
	if n := unsafe.Sizeof(Device{}); n > 96 {
		t.Fatalf("Device is %d B, budget 96 B", n)
	}
}
