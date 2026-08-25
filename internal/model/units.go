package model

import "math"

// KelvinToCelsius 开尔文转摄氏度。
func KelvinToCelsius(k float64) float64 { return k - 273.15 }

// CelsiusToKelvin 摄氏度转开尔文。
func CelsiusToKelvin(c float64) float64 { return c + 273.15 }

// ValidateTemperatureUnit 校验温度单位合法（C 或 K）。
func ValidateTemperatureUnit(u string) bool {
	return u == UnitCelsius || u == UnitKelvin
}

// CheckUnitConsistency 校验新曲线温度单位与试验一致；混用报 ErrMixedUnits。
// 同试验内禁止温度单位混用是核心不变量（并发与错误边界章节）。
func CheckUnitConsistency(trialUnit, curveUnit string) error {
	if trialUnit == "" {
		return nil
	}
	if !ValidateTemperatureUnit(curveUnit) {
		return E(ErrInvalidInput, "unsupported temperature unit %q", curveUnit)
	}
	if trialUnit != curveUnit {
		return E(ErrMixedUnits, "trial uses %s but curve provides %s", trialUnit, curveUnit)
	}
	return nil
}

// ValidateSamplingInterval 校验温度采样间隔：必须为正且不超过上限（10 K）。
// 采样过疏无法支撑峰检测的导数估计，故设置硬上限。
func ValidateSamplingInterval(interval float64) error {
	if interval <= 0 {
		return E(ErrInvalidInput, "sampling interval must be positive, got %v", interval)
	}
	if interval > 10.0 {
		return E(ErrSamplingTooCoarse, "sampling interval %v K exceeds 10 K max", interval)
	}
	return nil
}

// ValidateCurvePoints 校验曲线点列：至少两点、温度严格递增、值域有限。
// 温度倒序的曲线一律拒绝（并发与错误边界章节明确要求）。
func ValidateCurvePoints(points []Point) error {
	if len(points) < 2 {
		return ErrEmptyCurve
	}
	prev := points[0].Temp
	for i, p := range points {
		if math.IsNaN(p.Temp) || math.IsInf(p.Temp, 0) || math.IsNaN(p.Value) || math.IsInf(p.Value, 0) {
			return E(ErrInvalidInput, "point %d contains non-finite value", i)
		}
		if i > 0 && p.Temp <= prev {
			return E(ErrCurveUnsorted, "temperature at index %d (%v) not greater than previous (%v)", i, p.Temp, prev)
		}
		prev = p.Temp
	}
	return nil
}

// ResampleOnGrid 把曲线点列重采样到固定温度网格上，供跨曲线对齐
// （例如将 DSC 峰温度与 TGA 质量损失按温度对齐）使用。
// 网格范围取点列覆盖区间，步长为 gridStep；超出范围的点忽略。
func ResampleOnGrid(points []Point, gridStep float64) []Point {
	if len(points) < 2 || gridStep <= 0 {
		return nil
	}
	start := points[0].Temp
	end := points[len(points)-1].Temp
	var out []Point
	for t := start; t <= end; t += gridStep {
		out = append(out, Point{Temp: t, Value: Interp(points, t)})
	}
	return out
}

// Interp 在点列上做线性插值；t 超出范围时取端点值。
func Interp(points []Point, t float64) float64 {
	if len(points) == 0 {
		return 0
	}
	if t <= points[0].Temp {
		return points[0].Value
	}
	if t >= points[len(points)-1].Temp {
		return points[len(points)-1].Value
	}
	for i := 0; i < len(points)-1; i++ {
		if t >= points[i].Temp && t <= points[i+1].Temp {
			span := points[i+1].Temp - points[i].Temp
			if span == 0 {
				return points[i].Value
			}
			w := (t - points[i].Temp) / span
			return points[i].Value + w*(points[i+1].Value-points[i].Value)
		}
	}
	return points[len(points)-1].Value
}

// AlignMassLoss 计算 DCS 峰顶温度处对应的 TGA 相对质量变化（百分比）。
// tgaPoints 应为相对质量（%），返回峰顶处相对起点的质量损失。
func AlignMassLoss(tgaPoints []Point, peakTemp float64, refTemp float64) float64 {
	if len(tgaPoints) == 0 {
		return 0
	}
	ref := Interp(tgaPoints, refTemp)
	atPeak := Interp(tgaPoints, peakTemp)
	if ref == 0 {
		return 0
	}
	return (ref - atPeak) / math.Abs(ref) * 100
}
