package baseline

import (
	"math"

	"task233-thermopoly/internal/model"
)

// Quality 评估校正质量：返回基线校正后的残差统计，
// 用于判断是否应把分析段标记为 anomalous（异常）。
type Quality struct {
	RMS            float64 `json:"rms"`             // 扣除基线后信号的均方根
	PeakToPeak     float64 `json:"peak_to_peak"`    // 峰峰值
	DriftSlope     float64 `json:"drift_slope"`     // 残余线性漂移斜率
	IsAnomalous    bool    `json:"is_anomalous"`    // 是否异常
	Reason         string  `json:"reason,omitempty"`
}

// Evaluate 评估校正后的曲线：
// 1. RMS 过大（相对信号尺度）说明校正失败或曲线本身异常；
// 2. 残余线性漂移显著说明基线模型不匹配（如存在平台跳变）。
func Evaluate(corrected []model.Point, rawScale float64) Quality {
	if len(corrected) == 0 {
		return Quality{IsAnomalous: true, Reason: "empty corrected curve"}
	}
	sumSq := 0.0
	for _, p := range corrected {
		sumSq += p.Value * p.Value
	}
	rms := math.Sqrt(sumSq / float64(len(corrected)))
	// 用首尾 10% 段拟合残余漂移斜率
	cut := len(corrected) / 10
	if cut < 1 {
		cut = 1
	}
	first := meanWindow(corrected, 0, cut)
	last := meanWindow(corrected, len(corrected)-cut, cut)
	dt := corrected[len(corrected)-1].Temp - corrected[0].Temp
	slope := 0.0
	if dt != 0 {
		slope = (last - first) / dt
	}
	// 峰值
	minV, maxV := corrected[0].Value, corrected[0].Value
	for _, p := range corrected[1:] {
		if p.Value < minV {
			minV = p.Value
		}
		if p.Value > maxV {
			maxV = p.Value
		}
	}
	q := Quality{RMS: rms, PeakToPeak: maxV - minV, DriftSlope: slope}
	scale := rawScale
	if scale <= 0 {
		scale = 1
	}
	if rms > 10*scale {
		q.IsAnomalous = true
		q.Reason = "post-baseline RMS exceeds signal scale"
		return q
	}
	// 残余漂移绝对值超过信号尺度 20% 视为异常（TGA 平台跳变场景）
	if math.Abs(slope)*dt > 0.2*scale {
		q.IsAnomalous = true
		q.Reason = "residual linear drift exceeds 20% of signal scale"
		return q
	}
	return q
}

// DetectAnomaly 便捷入口：校正后调用，异常返回 true 与原因。
func DetectAnomaly(corrected []model.Point, rawScale float64) (bool, string) {
	q := Evaluate(corrected, rawScale)
	return q.IsAnomalous, q.Reason
}
