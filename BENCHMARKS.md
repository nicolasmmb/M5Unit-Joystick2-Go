# Benchmarks

The unit is I2C-only and pure Go, so every benchmark runs on the host with a
register-level fake (`fakeSensor` in `device_test.go`) that also emulates the
flash-backed address and calibration. Each benchmark reports **I2C
transactions per operation** (`tx/op`), the quantity that dominates on real
hardware: at 400 kHz one transaction costs roughly 60–80 µs.

## What is measured

| Benchmark | Realistic quantity |
|---|---|
| `BenchmarkSample` | one UI reading: position + button (**2 transactions**) |
| `BenchmarkPosition` | mapped position ±4095 (1 transaction) |
| `BenchmarkRaw` | 16-bit ADC X/Y (1 transaction) |
| `BenchmarkButton` | Z-axis button (1 transaction) |
| `BenchmarkLEDSet` / `BenchmarkBrightness` | LED write (1 transaction) |
| `BenchmarkLEDRead` | read back the LED register |
| `BenchmarkCalibration` / `BenchmarkSetCalibration` | 16-byte mapping block |
| `TestNoAlloc` | `AllocsPerRun == 0` for every operation |
| `TestFootprint` | `unsafe.Sizeof(Device{})` budget (96 B) |

## How to run

```bash
make test
make bench
make alloc
make bench-save NAME=before
# ... change the code ...
make bench-save NAME=after
make bench-compare FROM=before TO=after
make profile
make escape
```

## Results: first working version vs optimized (Apple M2 Pro, `benchstat -count=8`)

| Metric | Before | After | Change |
|---|---:|---:|---:|
| `Sample` tx/op | 3 | **2** | −33.3% |
| `Position` tx/op | 2 | **1** | −50.0% |
| `Raw` tx/op | 2 | **1** | −50.0% |
| `Sample` ns/op | 18.23 | **16.93** | −7.2% |
| `Position` ns/op | 15.82 | **14.10** | −10.8% |
| `Raw` ns/op | 16.96 | **14.17** | −16.5% |
| `Brightness` ns/op | 14.25 | **13.19** | −7.4% |
| allocations (all ops) | 0 | **0** | — |
| `Device` size | 72 B | **72 B** | — |

Current absolute numbers (all allocation-free):

| Benchmark | Result |
|---|---:|
| `Sample` | 16.9 ns, 2 tx |
| `Position` | 14.1 ns, 1 tx |
| `Raw` | 14.2 ns, 1 tx |
| `Button` | 12.8 ns, 1 tx |
| `LEDSet` | 13.4 ns, 1 tx |
| `Brightness` | 13.2 ns, 1 tx |
| `Calibration` | 15.4 ns, 1 tx |
| `SetCalibration` | 17.6 ns, 1 tx |

### What changed

1. **Position and raw ADC**: two 2-byte reads became one 4-byte burst, which
   halves the transactions of the UI path.
2. **`Sample`** now reads position (4 B) + button (1 B) in **2 transactions**
   instead of 3.
3. **LED brightness** uses a local color cache, so `SetBrightness` writes one
   transaction and never needs a read-modify-write. Brightness is applied
   exactly once (the UIFlow `fill_color(0xRRGGBB)` helper applies it twice).

### Why there is no Context API

Unlike the ToF driver, the Joystick2 has no polling loops: every call is one or
two I2C transactions and returns the current register value. There is nothing
to wait for besides the I2C transfer itself, which TinyGo bounds at 40 ms per
transaction. Lock contention is therefore bounded by at most two transfers.

Host numbers are Go on arm64. TinyGo/ESP32 executes the same fixed-buffer code;
the transaction counts and allocation counts carry over directly.
