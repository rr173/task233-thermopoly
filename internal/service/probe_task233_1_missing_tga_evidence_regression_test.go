package service

import (
	"testing"

	"task233-thermopoly/internal/model"
)

func TestBug01_MissingTGAEvidenceCannotConfirmPolymorph(t *testing.T) {
	svc := newTestService(t)
	tr, err := svc.CreateTrial(CreateTrialInput{Name: "missing-tga", Material: "FormA", Unit: model.UnitCelsius})
	if err != nil {
		t.Fatalf("create trial: %v", err)
	}
	if _, err := svc.ImportCurve(ImportCurveInput{TrialID: tr.ID, Kind: model.CurveDSC, Unit: model.UnitCelsius, Points: synthSmokeDSC()}); err != nil {
		t.Fatalf("import dsc: %v", err)
	}
	prior := model.PolymorphPrior{ID: "prior-1", FormFrom: "A", FormTo: "B", OnsetLow: 110, OnsetHigh: 130, Direction: model.DirectionEndothermic, MaxMassLossPct: 0.5, Active: true}
	if _, err := svc.CreatePrior(CreatePriorInput{FormFrom: prior.FormFrom, FormTo: prior.FormTo, OnsetLow: prior.OnsetLow, OnsetHigh: prior.OnsetHigh, Direction: prior.Direction, MaxMassLossPct: prior.MaxMassLossPct}); err != nil {
		t.Fatalf("create prior: %v", err)
	}
	if _, err := svc.RunBaseline(tr.ID); err != nil {
		t.Fatalf("baseline: %v", err)
	}
	if _, err := svc.DetectPeaks(tr.ID); err != nil {
		t.Fatalf("peaks: %v", err)
	}
	events, err := svc.GenerateEvents(tr.ID)
	if err != nil {
		t.Fatalf("service events: %v", err)
	}
	for _, event := range events {
		if event.Kind == model.EventPolymorph {
			t.Fatalf("service classified missing TGA evidence as polymorph: %+v", event)
		}
	}

	pipelineResult, err := svc.Pipeline().Run(
		[]model.Curve{{ID: "dsc", TrialID: "pipeline", Kind: model.CurveDSC, Points: synthSmokeDSC()}},
		nil,
		[]model.PolymorphPrior{prior},
	)
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	for _, event := range pipelineResult.Events {
		if event.Kind == model.EventPolymorph {
			t.Fatalf("pipeline classified missing TGA evidence as polymorph: %+v", event)
		}
	}
}
