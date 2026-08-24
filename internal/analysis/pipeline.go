// Package analysis 编排端到端分析流水线：基线校正 -> 峰检测 ->
// 晶型事件判读。它把 baseline/peak/event 三个算法包串成
// 可复现、可审计的单一入口（供 service 与 smoke-test 复用）。
package analysis

import (
	"task233-thermopoly/internal/baseline"
	"task233-thermopoly/internal/event"
	"task233-thermopoly/internal/model"
	"task233-thermopoly/internal/peak"
)

// Pipeline 是分析流水线的无状态执行器。
type Pipeline struct {
	baselineOpt baseline.Options
	peakOpt     peak.Options
}

// NewPipeline 创建流水线（参数可后续调整）。
func NewPipeline(b baseline.Options, p peak.Options) *Pipeline {
	return &Pipeline{baselineOpt: b, peakOpt: p}
}

// RunResult 是流水线单次执行的结果：校正段、峰列表与事件候选。
type RunResult struct {
	Segments []model.Segment `json:"segments"`
	Peaks    []model.Peak    `json:"peaks"`
	Events   []model.Event   `json:"events"`
}

// Run 对给定曲线集合执行完整流水线：
// 1. 对每条 DSC/TGA 曲线做基线校正并生成段；
// 2. 在 DSC 校正曲线上检测峰；
// 3. 用 TGA 对齐质量损失，结合晶型先验生成事件候选。
// 曲线顺序无关：DSC 曲线逐个检测，TGA 仅作为证据源。
func (p *Pipeline) Run(dscCurves []model.Curve, tgaCurves []model.Curve, priors []model.PolymorphPrior) (*RunResult, error) {
	res := &RunResult{}

	// 1. 基线校正
	for _, c := range dscCurves {
		cr, err := baseline.Correct(&c, p.baselineOpt)
		if err != nil {
			return nil, err
		}
		seg := model.Segment{
			ID:       "seg-" + c.ID,
			TrialID:  c.TrialID,
			CurveID:  c.ID,
			Kind:     c.Kind,
			Status:   model.SegmentBaselineCorrected,
			Baseline: baseline.ParamsJSON(map[string]any{"method": "linear"}),
			Params:   baseline.ParamsJSON(cr.Params),
		}
		res.Segments = append(res.Segments, seg)
	}

	// 2. 峰检测（在每条 DSC 校正曲线上）
	var tga []model.Curve = tgaCurves
	var allPeaks []model.Peak
	for _, c := range dscCurves {
		det, err := peak.Detect(&c, p.peakOpt)
		if err != nil {
			return nil, err
		}
		allPeaks = append(allPeaks, det.Peaks...)
	}
	res.Peaks = allPeaks

	// 3. 事件判读
	cl := event.NewClassifier(priors)
	firstTGA := firstCurve(tga)
	firstDSC := firstCurve(dscCurves)
	refTemp := 0.0
	if len(firstDSC.Points) > 0 {
		refTemp = firstDSC.Points[0].Temp
	}
	for _, pk := range allPeaks {
		massLoss := 0.0
		hasTGA := firstTGA != nil
		if hasTGA {
			massLoss = model.AlignMassLoss(firstTGA.Points, pk.PeakTemp, refTemp)
		}
		cand, err := cl.Classify(event.ClassifyInput{
			Peak:        pk,
			MassLossPct: massLoss,
			HasTGA:      hasTGA,
		})
		if err != nil {
			return nil, err
		}
		ev := cand.Event
		ev.ID = "evt-" + pk.ID
		ev.TrialID = pk.TrialID
		ev.Evidence = event.EvidenceJSON(cand.Evidence)
		// 重叠峰事件标记为 overlapping（不确定性）
		if pk.Overlap && ev.Status == model.EventCandidate {
			ev.Status = model.EventOverlapping
		}
		res.Events = append(res.Events, ev)
	}
	return res, nil
}

func firstCurve(curves []model.Curve) *model.Curve {
	if len(curves) == 0 {
		return nil
	}
	return &curves[0]
}
