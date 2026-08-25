// Package peak 实现热分析曲线的峰检测：平滑、导数估计、
// 峰区间切分、峰几何量计算与重叠判定（分谷深度）。
package peak

import "task233-thermopoly/internal/model"

// Options 是峰检测参数。
type Options struct {
	// SmoothWindow 移动平均平滑窗口（点数，默认 5，必须为奇数）。
	SmoothWindow int `json:"smooth_window"`
	// MinHeight 最小峰高阈值（信号单位），低于该值视为噪声。
	MinHeight float64 `json:"min_height"`
	// OverlapRatio 分谷深度阈值（0~1）：相邻峰谷值深度/较小峰高 < 阈值视为重叠。
	OverlapRatio float64 `json:"overlap_ratio"`
	// Direction 检测方向：positive（峰向上）| negative（峰向下）| both。
	Direction string `json:"direction"`
}

// DefaultOptions 返回默认检测参数。
// SmoothWindow 默认 3：窗口过大会把间距 6~8°C 的相邻峰抹平为单驼峰，
// 导致漏检（task233 smoke 合成峰 120/126°C 间距 6 实测窗口 5 漏检）。
func DefaultOptions() Options {
	return Options{
		SmoothWindow: 3,
		MinHeight:    0.05,
		OverlapRatio: 0.35,
		Direction:    "both",
	}
}

// Detected 是一次峰检测的结果。
type Detected struct {
	Peaks  []model.Peak `json:"peaks"`
	Params map[string]any `json:"params"`
}

// smooth 对纵坐标做窗口移动平均（边界截断），返回平滑后的纵坐标。
func smooth(values []float64, win int) []float64 {
	n := len(values)
	if win < 1 {
		win = 1
	}
	if win > n {
		win = n
	}
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		lo := i - win/2
		hi := i + win/2
		if lo < 0 {
			lo = 0
		}
		if hi >= n {
			hi = n - 1
		}
		sum := 0.0
		for j := lo; j <= hi; j++ {
			sum += values[j]
		}
		out[i] = sum / float64(hi-lo+1)
	}
	return out
}

// derivCentral 中心差分估计一阶导数：dy[i] = (y[i+1]-y[i-1])/(x[i+1]-x[i-1])。
func derivCentral(pts []model.Point) []float64 {
	n := len(pts)
	out := make([]float64, n)
	for i := 1; i < n-1; i++ {
		dt := pts[i+1].Temp - pts[i-1].Temp
		if dt == 0 {
			continue
		}
		out[i] = (pts[i+1].Value - pts[i-1].Value) / dt
	}
	return out
}
