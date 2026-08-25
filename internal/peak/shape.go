package peak

import (
	"math"

	"task233-thermopoly/internal/model"
)

// FWHM 计算峰半高宽（Full Width at Half Maximum）：以全局基线为基准，
// 峰高一半处左右边界的温度跨度。从峰顶向两侧寻找第一个越过半高线的点。
// 对重叠峰，若半高线不与相邻峰干扰则给出近似值，否则返回 0（不可靠信号）。
func FWHM(pts []model.Point, p model.Peak) float64 {
	if len(pts) == 0 {
		return 0
	}
	base := baselineValue(valuesOf(pts))
	half := base + math.Abs(p.PeakValue-base)/2
	apex := peakIdx(pts, p)
	left := p.PeakTemp
	right := p.PeakTemp
	for i := apex; i > 0; i-- {
		if crosses(pts[i].Value, pts[i-1].Value, half) {
			left = pts[i].Temp
			break
		}
	}
	for i := apex; i < len(pts)-1; i++ {
		if crosses(pts[i].Value, pts[i+1].Value, half) {
			right = pts[i+1].Temp
			break
		}
	}
	if right <= left {
		return 0
	}
	return right - left
}

// SuspectShoulder 判断峰是否疑似肩峰（右半宽显著大于左半宽，
// 或峰区间相对半高线不对称）。返回（疑似, 不对称度）。
func SuspectShoulder(pts []model.Point, p model.Peak) (bool, float64) {
	if len(pts) == 0 {
		return false, 1
	}
	base := baselineValue(valuesOf(pts))
	half := base + math.Abs(p.PeakValue-base)/2
	apex := peakIdx(pts, p)
	leftT, rightT := 0.0, 0.0
	for i := apex; i > 0; i-- {
		if crosses(pts[i].Value, pts[i-1].Value, half) {
			leftT = p.PeakTemp - pts[i].Temp
			break
		}
	}
	for i := apex; i < len(pts)-1; i++ {
		if crosses(pts[i].Value, pts[i+1].Value, half) {
			rightT = pts[i+1].Temp - p.PeakTemp
			break
		}
	}
	if rightT <= 0 {
		return true, math.Inf(1)
	}
	a := leftT / rightT
	return a > 2, a
}

// peakIdx 返回峰顶温度对应的索引。
func peakIdx(pts []model.Point, p model.Peak) int {
	for i := p.StartIdx; i <= p.EndIdx && i < len(pts); i++ {
		if pts[i].Temp >= p.PeakTemp {
			return i
		}
	}
	return p.StartIdx
}

func crosses(a, b, level float64) bool {
	return (a >= level && b <= level) || (a <= level && b >= level)
}

// MergeOverlapping 合并两个重叠峰为一个不确定区段（供事件层拆分前使用）。
// 返回合并后的峰区间（取并集）。
func MergeOverlapping(a, b model.Peak) model.Peak {
	m := a
	if b.StartIdx < m.StartIdx {
		m.StartIdx = b.StartIdx
		m.StartTemp = b.StartTemp
	}
	if b.EndIdx > m.EndIdx {
		m.EndIdx = b.EndIdx
		m.EndTemp = b.EndTemp
	}
	// 峰顶取信号更强者
	if abs(b.PeakValue) > abs(m.PeakValue) {
		m.PeakTemp = b.PeakTemp
		m.PeakValue = b.PeakValue
	}
	m.Overlap = true
	return m
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
