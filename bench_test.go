package joystick2

import "testing"

func reportTx(b *testing.B, f *fakeSensor, before int) {
	b.Helper()
	b.ReportMetric(float64(f.tx-before)/float64(b.N), "tx/op")
}

func BenchmarkSample(b *testing.B) {
	d, f := newTestDevice(b)
	b.ReportAllocs()
	b.ResetTimer()
	start := f.tx
	for i := 0; i < b.N; i++ {
		if _, err := d.Sample(); err != nil {
			b.Fatal(err)
		}
	}
	reportTx(b, f, start)
}

func BenchmarkPosition(b *testing.B) {
	d, f := newTestDevice(b)
	b.ReportAllocs()
	b.ResetTimer()
	start := f.tx
	for i := 0; i < b.N; i++ {
		if _, _, err := d.Position(); err != nil {
			b.Fatal(err)
		}
	}
	reportTx(b, f, start)
}

func BenchmarkRaw(b *testing.B) {
	d, f := newTestDevice(b)
	b.ReportAllocs()
	b.ResetTimer()
	start := f.tx
	for i := 0; i < b.N; i++ {
		if _, _, err := d.Raw(); err != nil {
			b.Fatal(err)
		}
	}
	reportTx(b, f, start)
}

func BenchmarkButton(b *testing.B) {
	d, f := newTestDevice(b)
	b.ReportAllocs()
	b.ResetTimer()
	start := f.tx
	for i := 0; i < b.N; i++ {
		if _, err := d.Button(); err != nil {
			b.Fatal(err)
		}
	}
	reportTx(b, f, start)
}

func BenchmarkLEDSet(b *testing.B) {
	d, f := newTestDevice(b)
	b.ReportAllocs()
	b.ResetTimer()
	start := f.tx
	for i := 0; i < b.N; i++ {
		if err := d.SetLED(Color{R: 10, G: 20, B: 30}); err != nil {
			b.Fatal(err)
		}
	}
	reportTx(b, f, start)
}

func BenchmarkBrightness(b *testing.B) {
	d, f := newTestDevice(b)
	_ = d.SetLED(Color{R: 200, G: 100, B: 50})
	b.ReportAllocs()
	b.ResetTimer()
	start := f.tx
	for i := 0; i < b.N; i++ {
		if err := d.SetBrightness(uint8(i % 101)); err != nil {
			b.Fatal(err)
		}
	}
	reportTx(b, f, start)
}

func BenchmarkLEDRead(b *testing.B) {
	d, f := newTestDevice(b)
	b.ReportAllocs()
	b.ResetTimer()
	start := f.tx
	for i := 0; i < b.N; i++ {
		if _, err := d.LED(); err != nil {
			b.Fatal(err)
		}
	}
	reportTx(b, f, start)
}

func BenchmarkCalibration(b *testing.B) {
	d, f := newTestDevice(b)
	b.ReportAllocs()
	b.ResetTimer()
	start := f.tx
	for i := 0; i < b.N; i++ {
		if _, err := d.Calibration(); err != nil {
			b.Fatal(err)
		}
	}
	reportTx(b, f, start)
}

func BenchmarkSetCalibration(b *testing.B) {
	d, f := newTestDevice(b)
	c := Calibration{
		XNegMin: 100, XNegMax: 30000, XPosMin: 35000, XPosMax: 65000,
		YNegMin: 200, YNegMax: 31000, YPosMin: 36000, YPosMax: 65000,
	}
	b.ReportAllocs()
	b.ResetTimer()
	start := f.tx
	for i := 0; i < b.N; i++ {
		if err := d.SetCalibration(c); err != nil {
			b.Fatal(err)
		}
	}
	reportTx(b, f, start)
}
