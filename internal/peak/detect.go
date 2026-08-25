package peak

import (
	"math"

	"task233-thermopoly/internal/model"
)

// Detect 执行完整峰检测流水线：
// 1. 平滑；2. 中心差分求导；3. 按导数过零定位峰顶（apex）；
// 4. 峰边界用导数极值法界定（上升段导最大点=起点，下降段导最小点=终点）；
// 5. 计算峰几何量（峰顶/高/面积，全局基线）；6. 过滤低于高度阈值的伪峰；
// 7. 相邻峰做重叠判定（分谷深度）。
func Detect(c *model.Curve, opt Options) (*Detected, error) {
	if err := model.ValidateCurvePoints(c.Points); err != nil {
		return nil, err
	}
	if opt.SmoothWindow == 0 {
		opt.SmoothWindow = DefaultOptions().SmoothWindow
	}
	if opt.SmoothWindow%2 == 0 {
		opt.SmoothWindow++ // 强制奇数窗口
	}
	if opt.MinHeight == 0 {
		opt.MinHeight = DefaultOptions().MinHeight
	}
	if opt.OverlapRatio == 0 {
		opt.OverlapRatio = DefaultOptions().OverlapRatio
	}
	if opt.Direction == "" {
		opt.Direction = DefaultOptions().Direction
	}

	pts := c.Points
	sm := smooth(valuesOf(pts), opt.SmoothWindow)
	dy := derivCentral(pointsWith(pts, sm))
	base := baselineValue(sm)
	threshold := opt.MinHeight

	var raw []model.Peak
	n := len(sm)
	for i := 1; i < n-1; i++ {
		dir := 0 // 0: none, 1: positive peak, -1: negative peak
		if dy[i-1] > 0 && dy[i] <= 0 && sm[i] > base+threshold {
			dir = 1
		} else if dy[i-1] < 0 && dy[i] >= 0 && sm[i] < base-threshold {
			dir = -1
		}
		if dir == 0 {
			continue
		}
		if opt.Direction == "positive" && dir < 0 {
			continue
		}
		if opt.Direction == "negative" && dir > 0 {
			continue
		}
		start, end := peakBounds(dy, sm, i, dir, n)
		raw = append(raw, model.Peak{
			StartIdx:  start,
			EndIdx:    end,
			StartTemp: pts[start].Temp,
			EndTemp:   pts[end].Temp,
			PeakTemp:  pts[i].Temp,
			PeakValue: sm[i],
			Direction: directionName(dir),
		})
	}

	// 计算几何量并过滤
	var peaks []model.Peak
	for _, p := range raw {
		if p.EndIdx-p.StartIdx < 2 {
			continue
		}
		h := math.Abs(p.PeakValue - base)
		if h < opt.MinHeight {
			continue
		}
		p.Height = h
		p.Area = trapezoidArea(pts, p.StartIdx, p.EndIdx, base)
		peaks = append(peaks, p)
	}

	// 重叠判定：相邻峰之间的谷值深度（两峰共享同一分离度，
	// 首个峰若无相邻前驱则保持默认 1，否则保留与后峰的共享值）。
	for i := 1; i < len(peaks); i++ {
		sep := valleySeparation(pts, peaks[i-1], peaks[i], base)
		peaks[i-1].Separation = sep
		peaks[i].Separation = sep
		if sep < opt.OverlapRatio {
			peaks[i-1].Overlap = true
			peaks[i].Overlap = true
		}
	}

	params := map[string]any{
		"smooth_window": opt.SmoothWindow,
		"min_height":    opt.MinHeight,
		"overlap_ratio": opt.OverlapRatio,
		"direction":     opt.Direction,
		"peaks_found":   len(peaks),
		"curve_hash":    c.Hash,
	}
	return &Detected{Peaks: peaks, Params: params}, nil
}

// peakBounds 用导数极值法界定峰边界：
// - 正峰：起点=上升段（dy>0）内 dy 最大点；终点=下降段（dy<0）内 dy 最小点。
// - 负峰：起点=下降段（dy<0）内 dy 最小点；终点=回升段（dy>0）内 dy 最大点。
// 对高斯峰，边界落在 ±σ 附近（最陡处），对邻近峰不越界。
func peakBounds(dy, sm []float64, apex, dir, n int) (int, int) {
	if dir > 0 {
		// 上升段左端
		j := apex
		for j > 1 && dy[j-1] > 0 {
			j--
		}
		start := j
		maxD := math.Inf(-1)
		for k := j; k <= apex; k++ {
			if dy[k] > maxD {
				maxD = dy[k]
				start = k
			}
		}
		// 下降段右端
		j = apex
		for j < n-2 && dy[j+1] < 0 {
			j++
		}
		end := j
		minD := math.Inf(1)
		for k := apex; k <= j; k++ {
			if dy[k] < minD {
				minD = dy[k]
				end = k
			}
		}
		return start, end
	}
	// 负峰
	j := apex
	for j > 1 && dy[j-1] < 0 {
		j--
	}
	start := j
	minD := math.Inf(1)
	for k := j; k <= apex; k++ {
		if dy[k] < minD {
			minD = dy[k]
			start = k
		}
	}
	j = apex
	for j < n-2 && dy[j+1] > 0 {
		j++
	}
	end := j
	maxD := math.Inf(-1)
	for k := apex; k <= j; k++ {
		if dy[k] > maxD {
			maxD = dy[k]
			end = k
		}
	}
	return start, end
}

func valuesOf(pts []model.Point) []float64 {
	out := make([]float64, len(pts))
	for i, p := range pts {
		out[i] = p.Value
	}
	return out
}

func pointsWith(pts []model.Point, values []float64) []model.Point {
	out := make([]model.Point, len(pts))
	for i, p := range pts {
		out[i] = model.Point{Temp: p.Temp, Value: values[i]}
	}
	return out
}

// baselineValue 以曲线首尾均值作为峰检测的全局基线参考。
func baselineValue(sm []float64) float64 {
	n := len(sm)
	if n == 0 {
		return 0
	}
	return (sm[0] + sm[n-1]) / 2
}

func directionName(dir int) string {
	if dir > 0 {
		return model.DirectionEndothermic
	}
	return model.DirectionExothermic
}

// trapezoidArea 梯形积分计算峰面积（扣除全局基线下面积）。
func trapezoidArea(pts []model.Point, start, end int, base float64) float64 {
	area := 0.0
	for i := start; i < end; i++ {
		dt := pts[i+1].Temp - pts[i].Temp
		avg := (pts[i].Value + pts[i+1].Value) / 2
		area += (avg - base) * dt
	}
	if area < 0 {
		area = -area
	}
	return area
}

// valleySeparation 计算两相邻峰之间的分谷深度：
// 谷值相对两峰中较矮峰的回落比例，1=完全分离，0=完全未分离。
func valleySeparation(pts []model.Point, a, b model.Peak, base float64) float64 {
	lo := a.EndIdx
	hi := b.StartIdx
	if lo > hi {
		lo, hi = hi, lo
	}
	if lo < a.StartIdx {
		lo = a.StartIdx
	}
	if hi > b.EndIdx {
		hi = b.EndIdx
	}
	if hi <= lo {
		return 1
	}
	valley := math.Inf(1)
	for i := lo; i <= hi; i++ {
		v := math.Abs(pts[i].Value - base)
		if v < math.Abs(valley-base) {
			valley = pts[i].Value
		}
	}
	hA := math.Abs(a.PeakValue - base)
	hB := math.Abs(b.PeakValue - base)
	minH := hA
	if hB < minH {
		minH = hB
	}
	if minH <= 1e-12 {
		return 1
	}
	drop := math.Abs(valley-base) / minH
	if drop > 1 {
		drop = 1
	}
	return 1 - drop
}
