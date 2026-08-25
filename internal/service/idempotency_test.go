package service

import (
	"testing"

	"task233-thermopoly/internal/model"
)

func TestAnalysisRequestsAreIdempotent(t *testing.T) {
	svc := newTestService(t)
	tr, err := svc.CreateTrial(CreateTrialInput{Name: "idempotent", Material: "FormA", Unit: model.UnitCelsius})
	if err != nil {
		t.Fatalf("create trial: %v", err)
	}
	if _, err := svc.ImportCurve(ImportCurveInput{TrialID: tr.ID, Kind: model.CurveDSC, Unit: model.UnitCelsius, Points: synthSmokeDSC()}); err != nil {
		t.Fatalf("import dsc: %v", err)
	}
	if _, err := svc.ImportCurve(ImportCurveInput{TrialID: tr.ID, Kind: model.CurveTGA, Unit: model.UnitCelsius, Points: synthSmokeTGA()}); err != nil {
		t.Fatalf("import tga: %v", err)
	}
	if _, err := svc.CreatePrior(CreatePriorInput{
		FormFrom: "FormA", FormTo: "FormB", OnsetLow: 110, OnsetHigh: 130,
		Direction: model.DirectionEndothermic, MaxMassLossPct: 0.5,
	}); err != nil {
		t.Fatalf("create prior: %v", err)
	}
	if _, err := svc.RunBaseline(tr.ID); err != nil {
		t.Fatalf("baseline: %v", err)
	}
	firstPeaks, err := svc.DetectPeaks(tr.ID)
	if err != nil {
		t.Fatalf("first peak detection: %v", err)
	}
	secondPeaks, err := svc.DetectPeaks(tr.ID)
	if err != nil {
		t.Fatalf("second peak detection: %v", err)
	}
	if len(firstPeaks) != 2 || len(secondPeaks) != len(firstPeaks) {
		t.Fatalf("peak detection is not idempotent: first=%d second=%d", len(firstPeaks), len(secondPeaks))
	}

	firstEvents, err := svc.GenerateEvents(tr.ID)
	if err != nil {
		t.Fatalf("first event generation: %v", err)
	}
	secondEvents, err := svc.GenerateEvents(tr.ID)
	if err != nil {
		t.Fatalf("second event generation: %v", err)
	}
	if len(firstEvents) != 2 || len(secondEvents) != len(firstEvents) {
		t.Fatalf("event generation is not idempotent: first=%d second=%d", len(firstEvents), len(secondEvents))
	}
}
