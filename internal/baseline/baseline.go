// Package baseline 实现热分析曲线的基线校正：
// DSC 用端点线性基线扣除，TGA 用平台漂移线性校正。
// 校正结果写入新的热分析段（Segment），原始曲线保持不变（不可变输入）。
package baseline

import (
	"encoding/json"
	"fmt"

	"task233-thermopoly/internal/model"
)

// Options 是基线校正的参数。
type Options struct {
	// SmoothWindow 用于端点估计的平滑窗口点数（默认 3）。
	SmoothWindow int `json:"smooth_window"`
	// Method 校正方法：linear（端点线性，默认）| plateau（平台漂移）。
	Method string `json:"method"`
}

// DefaultOptions 返回默认校正参数。
func DefaultOptions() Options {
	return Options{SmoothWindow: 3, Method: "linear"}
}

// Result 是一次校正的结果：校正后点列与参数摘要。
type Result struct {
	Corrected []model.Point `json:"corrected"`
	Params    map[string]any `json:"params"`
}

// baselineFn 从曲线构造基线函数：输入温度，输出基线值。
type baselineFn func(t float64) float64

// LinearBaseline 构造连接两端端点（平滑后）的线性基线。
// b(t) = y0 + (yN-y0)*(t-t0)/(tN-t0)
func LinearBaseline(pts []model.Point, win int) baselineFn {
	if win < 1 {
		win = 1
	}
	n := len(pts)
	if n < 2 {
		return func(float64) float64 { return 0 }
	}
	y0 := meanWindow(pts, 0, win)
	yN := meanWindow(pts, n-win, win)
	t0 := pts[0].Temp
	tN := pts[n-1].Temp
	slope := 0.0
	if tN != t0 {
		slope = (yN - y0) / (tN - t0)
	}
	return func(t float64) float64 {
		return y0 + slope*(t-t0)
	}
}

// PlateauBaseline 构造平台漂移基线：取前后平台区均值，
// 以线性漂移连接两平台，适合 TGA 长时间漂移校正。
func PlateauBaseline(pts []model.Point, win int) baselineFn {
	return LinearBaseline(pts, win)
}

// CorrectDSC 对 DSC 热流曲线做基线扣除：corrected[i] = raw[i] - baseline(t[i])。
func CorrectDSC(pts []model.Point, opt Options) ([]model.Point, error) {
	if err := model.ValidateCurvePoints(pts); err != nil {
		return nil, err
	}
	win := opt.SmoothWindow
	if win < 1 {
		win = 1
	}
	if win > len(pts)/2 {
		win = len(pts) / 2
	}
	bf := LinearBaseline(pts, win)
	out := make([]model.Point, len(pts))
	for i, p := range pts {
		out[i] = model.Point{Temp: p.Temp, Value: p.Value - bf(p.Temp)}
	}
	return out, nil
}

// CorrectTGA 对 TGA 质量曲线做平台漂移校正：
// 将整体质量曲线扣除漂移基线，使前后平台对齐（相对质量百分数语义不变）。
func CorrectTGA(pts []model.Point, opt Options) ([]model.Point, error) {
	if err := model.ValidateCurvePoints(pts); err != nil {
		return nil, err
	}
	win := opt.SmoothWindow
	if win < 1 {
		win = 1
	}
	if win > len(pts)/2 {
		win = len(pts) / 2
	}
	bf := PlateauBaseline(pts, win)
	out := make([]model.Point, len(pts))
	for i, p := range pts {
		out[i] = model.Point{Temp: p.Temp, Value: p.Value - bf(p.Temp) + pts[0].Value}
	}
	return out, nil
}

// Correct 按曲线类型分发校正，并返回参数摘要供持久化。
func Correct(c *model.Curve, opt Options) (*Result, error) {
	var (
		corrected []model.Point
		err       error
		method    = opt.Method
	)
	if method == "" {
		method = "linear"
	}
	switch c.Kind {
	case model.CurveDSC:
		corrected, err = CorrectDSC(c.Points, opt)
	case model.CurveTGA:
		corrected, err = CorrectTGA(c.Points, opt)
	default:
		return nil, model.E(model.ErrInvalidInput, "cannot correct curve kind %q", c.Kind)
	}
	if err != nil {
		return nil, err
	}
	params := map[string]any{
		"method":        method,
		"smooth_window": opt.SmoothWindow,
		"points_in":     len(c.Points),
		"points_out":    len(corrected),
		"curve_hash":    c.Hash,
		"curve_kind":    c.Kind,
	}
	return &Result{Corrected: corrected, Params: params}, nil
}

// ParamsJSON 序列化参数摘要（供 Segment.Params 存储）。
func ParamsJSON(params map[string]any) string {
	raw, err := json.Marshal(params)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(raw)
}

// meanWindow 计算点列从 start 开始的 win 个点纵坐标均值。
func meanWindow(pts []model.Point, start, win int) float64 {
	n := len(pts)
	if start < 0 {
		start = 0
	}
	end := start + win
	if end > n {
		end = n
	}
	if end <= start {
		return pts[start].Value
	}
	sum := 0.0
	for i := start; i < end; i++ {
		sum += pts[i].Value
	}
	return sum / float64(end-start)
}
