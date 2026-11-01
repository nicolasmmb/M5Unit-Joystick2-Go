# M5Unit-Joystick2-Go

[![test](https://github.com/nicolasmmb/M5Unit-Joystick2-Go/actions/workflows/test.yml/badge.svg)](https://github.com/nicolasmmb/M5Unit-Joystick2-Go/actions/workflows/test.yml)

Pure-Go driver for the **M5Stack Unit Joystick2** (SKU U024-V2): a Hall-effect
3-axis joystick with an STM32G030, I2C interface and a WS2812C RGB LED.
No C, C++, CGo or Arduino.

## Features

- 16-bit raw ADC (0–65535), 8-bit raw ADC, mapped position (±4095) and 8-bit
  mapped position (±127), plus the Z-axis button.
- **2 I2C transactions per UI sample** (position + button).
- RGB LED with a local color/brightness cache: one write per change, and
  brightness applied exactly once.
- Full calibration parity: read/write the 8× u16 mapping and set deadzones in
  ADC or position units.
- Firmware/bootloader version and I2C address (read and change).
- Orientation helpers (invert X/Y, swap axes) applied by the driver.
- **Zero heap allocations per operation**: fixed scratch buffers, no `fmt`.
- Host tests with a register/flash emulator, transaction-count contracts, RAM
  footprint budget and benchmarks.

## Hardware

| Parameter | Value |
|---|---|
| Unit | M5Stack Unit Joystick2 (U024-V2) |
| MCU | STM32G030F6P6 (64 MHz, 32 KB flash / 8 KB SRAM) |
| Interface | I2C, **0x63**; use **100 kHz** (400 kHz timed out with this firmware) |
| X/Y | Hall effect, 16-bit ADC (0–65535), mapped to ±4095 |
| Z | push button (register 1 = released, **0 = pressed**) |
| LED | 1× WS2812C RGB |
| Wiring (M5StickC) | Grove I2C1: **SDA=G32, SCL=G33** |

The Grove port is shared with the other units (Encoder, GPS, Fingerprint2,
ToF): connect only one at a time.

## Install

```bash
go get github.com/nicolasmmb/M5Unit-Joystick2-Go@v0.1.0
```

## Quick start

```go
i2c := machine.I2C1
_ = i2c.Configure(machine.I2CConfig{
    SDA:       machine.GPIO32,
    SCL:       machine.GPIO33,
    Frequency: 100 * machine.KHz, // 400 kHz timed out on the STM32 slave
})

d := joystick2.New(i2c)
if !d.Present() {
    println("joystick2 not found")
}
_ = d.SetLED(joystick2.Color{R: 0, G: 64, B: 64})
_ = d.SetBrightness(50) // brightness applied once

for {
    s, err := d.Sample() // 2 I2C transactions
    if err == nil {
        println(s.X, s.Y, s.Pressed)
    }
    time.Sleep(20 * time.Millisecond)
}
```

## API

| Function | Description |
|---|---|
| `New(bus I2C)` / `NewAt(bus, addr)` | Driver on any bus implementing `Tx(addr, w, r)` |
| `Present() bool` | Does the unit answer on the bus? |
| `FirmwareVersion()` / `BootloaderVersion()` | Version registers |
| `Address() uint8` / `SetAddress(addr) error` | Address (change persists in flash) |
| `Raw() (x, y uint16, error)` | 16-bit ADC, 0–65535 |
| `Raw8() (x, y uint8, error)` | 8-bit ADC |
| `Button() (bool, error)` | Z-axis button (`true` = pressed) |
| `Position() (x, y int16, error)` | Mapped position, ±4095 |
| `Position8() (x, y int8, error)` | Mapped position, ±127 |
| `Sample() (Sample, error)` | Position + button in 2 transactions |
| `SetOrientation(o)` / `Orientation()` | Invert X/Y and swap axes |
| `SetLED(Color)` / `LED()` / `SetBrightness(0-100)` | RGB LED and cached brightness |
| `Calibration()` / `SetCalibration(Cal)` | 8× u16 mapping (flash-backed) |
| `SetAxisMapping(axis, m)` | One axis (8 bytes) |
| `SetDeadzoneADC(x, y)` / `SetDeadzonePosition(x, y)` | Deadzone via mapping |

`Sample`: `X, Y int16`, `Pressed bool`. `Color`: `R, G, B uint8`.
`Calibration`: `XNegMin, XNegMax, XPosMin, XPosMax, YNegMin, YNegMax, YPosMin, YPosMax uint16`.

## Register map (from the official firmware)

| Register | Content |
|---|---|
| `0x00..0x03` | raw ADC X/Y, u16 LE (0–65535) |
| `0x10..0x11` | raw ADC X/Y, 8-bit |
| `0x20` | button (1 released, 0 pressed) |
| `0x30..0x33` | LED u32 LE: B, G, R, A (A unused) |
| `0x40..0x4F` | calibration: X/Y neg min/max, pos min/max (u16 LE) |
| `0x50..0x53` | mapped position X/Y, int16 LE (±4095) |
| `0x60..0x61` | mapped position, int8 (±127) |
| `0xFC` / `0xFE` | bootloader / firmware version |
| `0xFF` | I2C address (write to change, persisted) |

## LED semantics

The register stores **B, G, R, A**. `SetLED` writes B, G, R scaled by the
current brightness and keeps the unscaled color in memory; `SetBrightness`
rewrites it. The official UIFlow `fill_color(0xRRGGBB)` helper applies the
brightness twice (once in `color_to_bgr` and again when writing); this driver
applies it once, like `fill_color_rgb`.

## Flash-backed settings

The STM32 persists the **calibration/deadzone and the I2C address** in its
flash on every write. `SetDeadzone*` leaves 100 ms between the X and Y writes
(matching the official driver), and `SetAddress` waits 10 ms for the firmware
to restart its I2C peripheral. Do not rewrite the mapping in a tight loop: flash
endurance is limited.

## I2C speed

The STM32G030 firmware is an interrupt-driven I2C slave. On the bench, polling
continuously at **400 kHz** produced `i2c: timeout during write` errors, while
**100 kHz** (the speed used by the UIFlow examples) ran at 35–43 Hz with zero
timeouts. The driver works at either speed; 100 kHz is recommended.

## Timeouts and cancellation

There are no polling loops: every call is one or two I2C transactions that
return the current register value. TinyGo bounds each transaction at 40 ms, and
a call waits at most for the other operation holding the device lock (one
transfer). No `context` API is needed.

## Performance

Measured on the host against a register-level fake; `tx/op` is the real cost at
400 kHz. See [BENCHMARKS.md](BENCHMARKS.md).

| Operation | I2C transactions | Allocations |
|---|---:|---:|
| `Sample` (position + button) | **2** | 0 |
| `Position` / `Raw` | **1** | 0 |
| `Button` / `LED` / `SetLED` / `Brightness` | 1 | 0 |
| `Calibration` / `SetCalibration` | 1 | 0 |

`Device` is **72 bytes**; `Color` is 3 bytes, `Sample` 6 bytes, `Calibration`
16 bytes.

```bash
make bench    # benchmarks with tx/op and allocs
make alloc    # 0-allocation and footprint contracts
```

## References

- [M5Stack Unit Joystick2](https://docs.m5stack.com/en/unit/Unit_Joystick2)
- [M5Stack UIFlow driver](https://github.com/m5stack/uiflow-micropython/blob/master/m5stack/libs/unit/joystick2.py)
- [M5Unit-Joystick2 (Arduino)](https://github.com/m5stack/M5Unit-Joystick2)
- [Internal firmware](https://github.com/m5stack/M5Unit-Joystick2-Internal-FW)

## License

MIT — see [LICENSE](LICENSE).
