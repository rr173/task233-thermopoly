package model

import (
	"errors"
	"testing"
)

func TestCheckUnitConsistency(t *testing.T) {
	if err := CheckUnitConsistency(UnitCelsius, UnitKelvin); err == nil {
		t.Error("mixed units must be rejected")
	}
	if !IsKind(CheckUnitConsistency(UnitCelsius, UnitKelvin), ErrMixedUnits) {
		t.Error("error kind must be ErrMixedUnits")
	}
	if err := CheckUnitConsistency(UnitCelsius, UnitCelsius); err != nil {
		t.Errorf("same unit rejected: %v", err)
	}
}

func TestValidateSamplingInterval(t *testing.T) {
	if err := ValidateSamplingInterval(0); err == nil {
		t.Error("zero interval must fail")
	}
	if err := ValidateSamplingInterval(12); err == nil {
		t.Error("coarse interval must fail")
	}
	if !IsKind(ValidateSamplingInterval(12), ErrSamplingTooCoarse) {
		t.Error("must be ErrSamplingTooCoarse")
	}
	if err := ValidateSamplingInterval(1.0); err != nil {
		t.Errorf("fine interval rejected: %v", err)
	}
}

func TestValidateCurvePoints(t *testing.T) {
	pts := []Point{{Temp: 30, Value: 1}, {Temp: 31, Value: 2}, {Temp: 32, Value: 1.5}}
	if err := ValidateCurvePoints(pts); err != nil {
		t.Errorf("valid curve rejected: %v", err)
	}
	bad := []Point{{Temp: 30, Value: 1}, {Temp: 30, Value: 2}}
	if err := ValidateCurvePoints(bad); err == nil {
		t.Error("non-increasing temperatures must fail")
	}
	if !IsKind(ValidateCurvePoints(bad), ErrCurveUnsorted) {
		t.Error("must be ErrCurveUnsorted")
	}
	single := []Point{{Temp: 30, Value: 1}}
	if err := ValidateCurvePoints(single); !errors.Is(err, ErrEmptyCurve) {
		t.Error("single point must be ErrEmptyCurve")
	}
}

func TestCanTransition(t *testing.T) {
	if !CanTransition(TrialReceiving, TrialConfirmed) {
		t.Error("receiving -> confirmed must be allowed")
	}
	if CanTransition(TrialConfirmed, TrialReceiving) {
		t.Error("backward transition must be rejected")
	}
	if CanTransition(TrialSealed, TrialPending) {
		t.Error("sealed is terminal")
	}
	if !CanTransition(TrialPending, TrialNeedsReview) {
		t.Error("pending -> needs_review must be allowed")
	}
}

func TestAlignMassLoss(t *testing.T) {
	// TGA：100% 直到 120°C，之后每 °C 降 1%
	var pts []Point
	for temp := 30.0; temp <= 200.0; temp += 1.0 {
		v := 100.0
		if temp > 120 {
			v = 100.0 - (temp - 120)
		}
		pts = append(pts, Point{Temp: temp, Value: v})
	}
	// 峰顶 126°C -> 损失 ≈6%
	loss := AlignMassLoss(pts, 126, 30)
	if loss < 5.5 || loss > 6.5 {
		t.Errorf("mass loss = %v, want ~6", loss)
	}
	// 峰顶 100°C -> 无损失
	loss0 := AlignMassLoss(pts, 100, 30)
	if loss0 > 0.5 {
		t.Errorf("mass loss at 100C = %v, want ~0", loss0)
	}
}

func TestResampleOnGrid(t *testing.T) {
	pts := []Point{{Temp: 30, Value: 0}, {Temp: 31, Value: 10}, {Temp: 32, Value: 20}}
	out := ResampleOnGrid(pts, 0.5)
	if len(out) == 0 {
		t.Fatal("empty resample")
	}
	if out[1].Temp != 30.5 || out[1].Value != 5 {
		t.Errorf("resample mismatch: %+v", out[1])
	}
}
