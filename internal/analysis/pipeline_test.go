package analysis

import (
	"math"
	"testing"

	"task233-thermopoly/internal/baseline"
	"task233-thermopoly/internal/model"
	"task233-thermopoly/internal/peak"
)

func TestPipelineAnchorsMassLossToTGAStart(t *testing.T) {
	dsc := model.Curve{
		ID: "dsc-1", TrialID: "trial-1", Kind: model.CurveDSC, Hash: "dsc-hash",
		Points: []model.Point{
			{Temp: 20, Value: 0}, {Temp: 120, Value: 0}, {Temp: 130, Value: 10},
			{Temp: 140, Value: 0}, {Temp: 200, Value: 0},
		},
	}
	tga := model.Curve{
		ID: "tga-1", TrialID: "trial-1", Kind: model.CurveTGA, Hash: "tga-hash",
		Points: []model.Point{
			{Temp: 100, Value: 100}, {Temp: 120, Value: 100}, {Temp: 130, Value: 98},
			{Temp: 150, Value: 98}, {Temp: 200, Value: 98},
		},
	}
	prior := model.PolymorphPrior{
		ID: "prior-1", FormFrom: "A", FormTo: "B", OnsetLow: 125, OnsetHigh: 135,
		Direction: model.DirectionEndothermic, MaxMassLossPct: 0.5, Active: true,
	}
	result, err := NewPipeline(baseline.DefaultOptions(), peak.DefaultOptions()).Run([]model.Curve{dsc}, []model.Curve{tga}, []model.PolymorphPrior{prior})
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	if len(result.Events) != 1 {
		t.Fatalf("events = %d, want 1", len(result.Events))
	}
	if result.Events[0].Kind != model.EventDesolvation {
		t.Fatalf("event kind = %s, want desolvation", result.Events[0].Kind)
	}
	// 非重叠峰不应把试验带入需复核
	if result.NeedsReview {
		t.Fatalf("needs_review = true for non-overlapping peak, want false")
	}
}

// TestPipelineMarksOverlappingEventsNeedsReview 覆盖 DSC 双峰重叠的不确定性链路：
// 峰检测标 Overlap -> 两条事件都置为 overlapping -> RunResult.NeedsReview 置位。
// 该链路曾因 pipeline 丢弃 Overlap 而退化为普通待判读（结果看似 candidate，
// 试验不进 needs_review），此测试锁定其不再回退。
func TestPipelineMarksOverlappingEventsNeedsReview(t *testing.T) {
	// 两个高斯吸热峰（120/126°C，σ=3）谷值浅，构成重叠对。
	var pts []model.Point
	for temp := 30.0; temp <= 200.0; temp += 1.0 {
		d1 := (temp - 120) / 3.0
		d2 := (temp - 126) / 3.0
		pts = append(pts, model.Point{Temp: temp, Value: 1.0*math.Exp(-d1*d1) + 0.8*math.Exp(-d2*d2)})
	}
	dsc := model.Curve{
		ID: "dsc-1", TrialID: "trial-1", Kind: model.CurveDSC, Hash: "dsc-hash", Points: pts,
	}
	tga := model.Curve{
		ID: "tga-1", TrialID: "trial-1", Kind: model.CurveTGA, Hash: "tga-hash",
		Points: []model.Point{{Temp: 30, Value: 100}, {Temp: 200, Value: 100}}, // 无失重：峰不会被判为脱除
	}
	prior := model.PolymorphPrior{
		ID: "prior-1", FormFrom: "A", FormTo: "B", OnsetLow: 110, OnsetHigh: 130,
		Direction: model.DirectionEndothermic, MaxMassLossPct: 0.5, Active: true,
	}
	result, err := NewPipeline(baseline.DefaultOptions(), peak.DefaultOptions()).Run([]model.Curve{dsc}, []model.Curve{tga}, []model.PolymorphPrior{prior})
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	if len(result.Peaks) != 2 {
		t.Fatalf("peaks = %d, want 2 overlapping pair", len(result.Peaks))
	}
	for i, p := range result.Peaks {
		if !p.Overlap {
			t.Errorf("peak %d overlap = false, want true (uncertainty must surface at peak layer)", i)
		}
	}
	if len(result.Events) != 2 {
		t.Fatalf("events = %d, want 2", len(result.Events))
	}
	for i, e := range result.Events {
		if e.Status != model.EventOverlapping {
			t.Errorf("event %d status = %s, want overlapping (both overlapping peaks must be flagged for review)", i, e.Status)
		}
	}
	if !result.NeedsReview {
		t.Fatalf("needs_review = false, want true: overlapping must propagate to trial-level uncertainty")
	}
}
