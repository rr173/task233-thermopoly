package peak

import (
	"math"
	"testing"

	"task233-thermopoly/internal/model"
)

// synthGauss 构造单个高斯峰曲线（用于单元测试）。
func synthGauss(center, amp, sigma, base float64) []model.Point {
	var pts []model.Point
	for temp := 20.0; temp <= 200.0; temp += 1.0 {
		d := (temp - center) / sigma
		pts = append(pts, model.Point{Temp: temp, Value: base + amp*math.Exp(-d*d)})
	}
	return pts
}

func TestDetectSinglePeak(t *testing.T) {
	c := &model.Curve{Points: synthGauss(120, 1.0, 3.0, 0), Kind: model.CurveDSC}
	det, err := Detect(c, DefaultOptions())
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if len(det.Peaks) != 1 {
		t.Fatalf("expected 1 peak, got %d", len(det.Peaks))
	}
	p := det.Peaks[0]
	if math.Abs(p.PeakTemp-120) > 1.5 {
		t.Errorf("peak temp = %v, want ~120", p.PeakTemp)
	}
	if p.Height < 0.8 {
		t.Errorf("height = %v, want > 0.8", p.Height)
	}
	if p.Overlap {
		t.Error("single peak must not be marked overlapping")
	}
}

func TestDetectOverlappingPeaks(t *testing.T) {
	var pts []model.Point
	for temp := 20.0; temp <= 200.0; temp += 1.0 {
		d1 := (temp - 120) / 3.0
		d2 := (temp - 126) / 3.0
		v := 1.0*math.Exp(-d1*d1) + 0.8*math.Exp(-d2*d2)
		pts = append(pts, model.Point{Temp: temp, Value: v})
	}
	c := &model.Curve{Points: pts, Kind: model.CurveDSC}
	det, err := Detect(c, DefaultOptions())
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if len(det.Peaks) != 2 {
		t.Fatalf("expected 2 peaks, got %d", len(det.Peaks))
	}
	if !det.Peaks[0].Overlap || !det.Peaks[1].Overlap {
		t.Errorf("expected both overlapping, got %v/%v (sep=%v)",
			det.Peaks[0].Overlap, det.Peaks[1].Overlap, det.Peaks[0].Separation)
	}
	if det.Peaks[0].Separation >= 0.35 {
		t.Errorf("separation = %v, want < 0.35 for overlap", det.Peaks[0].Separation)
	}
}

func TestDetectRejectsUnsorted(t *testing.T) {
	c := &model.Curve{Points: []model.Point{
		{Temp: 50, Value: 0}, {Temp: 40, Value: 0.1}, {Temp: 60, Value: 0},
	}}
	if _, err := Detect(c, DefaultOptions()); err == nil {
		t.Fatal("expected error for unsorted temperatures")
	}
}

func TestFWHM(t *testing.T) {
	pts := synthGauss(120, 1.0, 3.0, 0)
	p := model.Peak{StartIdx: 0, EndIdx: len(pts) - 1, PeakTemp: 120, PeakValue: 1.0}
	fw := FWHM(pts, p)
	// 合成函数 exp(-(d/s)^2) 的半高宽 = 2*s*sqrt(ln2) ≈ 2*3*0.8326 ≈ 5.0
	if math.Abs(fw-5.0) > 1.0 {
		t.Errorf("FWHM = %v, want ~5.0", fw)
	}
}
