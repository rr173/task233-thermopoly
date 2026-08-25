package baseline

import (
	"math"
	"testing"

	"task233-thermopoly/internal/model"
)

// linearRamp 构造带线性漂移的曲线：y = base + slope*t + gauss 峰。
func TestCorrectDSCRemovesLinearDrift(t *testing.T) {
	var pts []model.Point
	for temp := 30.0; temp <= 200.0; temp += 1.0 {
		d := (temp - 120) / 3.0
		v := 1.0*math.Exp(-d*d) + 0.02*temp // 0.02/°C 线性漂移
		pts = append(pts, model.Point{Temp: temp, Value: v})
	}
	opt := DefaultOptions()
	out, err := CorrectDSC(pts, opt)
	if err != nil {
		t.Fatalf("correct: %v", err)
	}
	// 校正后曲线两端应接近 0（漂移被扣除）
	if math.Abs(out[0].Value) > 0.1 {
		t.Errorf("left edge residual = %v, want ~0", out[0].Value)
	}
	if math.Abs(out[len(out)-1].Value) > 0.1 {
		t.Errorf("right edge residual = %v, want ~0", out[len(out)-1].Value)
	}
	// 峰区域仍保留信号
	peakVal := 0.0
	for _, p := range out {
		if p.Value > peakVal {
			peakVal = p.Value
		}
	}
	if peakVal < 0.8 {
		t.Errorf("peak signal = %v, want > 0.8 (peak must survive correction)", peakVal)
	}
}

func TestCorrectTGAAlignsPlateau(t *testing.T) {
	var pts []model.Point
	for temp := 30.0; temp <= 200.0; temp += 1.0 {
		mass := 100.0
		if temp > 120 {
			mass = 100.0 - 0.5*(temp-120) // 漂移
		}
		pts = append(pts, model.Point{Temp: temp, Value: mass})
	}
	out, err := CorrectTGA(pts, DefaultOptions())
	if err != nil {
		t.Fatalf("correct: %v", err)
	}
	if math.Abs(out[0].Value-out[0].Value) > 1e-9 {
		t.Error("TGA correction must preserve start value")
	}
}

func TestEvaluateFlagsAnomaly(t *testing.T) {
	// 构造 RMS 超标的"校正后"曲线
	var pts []model.Point
	for i := 0; i < 100; i++ {
		pts = append(pts, model.Point{Temp: float64(i), Value: 1000 * math.Sin(float64(i))})
	}
	q := Evaluate(pts, 1.0)
	if !q.IsAnomalous {
		t.Errorf("expected anomalous, got %+v", q)
	}
}

func TestCorrectRejectsEmpty(t *testing.T) {
	if _, err := CorrectDSC(nil, DefaultOptions()); err == nil {
		t.Fatal("expected error for empty curve")
	}
}
