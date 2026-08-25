package analysis

import (
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
}
