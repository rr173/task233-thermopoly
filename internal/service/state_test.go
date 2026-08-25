package service

import (
	"testing"

	"task233-thermopoly/internal/model"
)

func TestVetoedEventKeepsTrialInReview(t *testing.T) {
	svc := newTestService(t)
	tr, err := svc.CreateTrial(CreateTrialInput{Name: "veto", Material: "FormA", Unit: model.UnitCelsius})
	if err != nil {
		t.Fatalf("create trial: %v", err)
	}
	if _, err := svc.ImportCurve(ImportCurveInput{TrialID: tr.ID, Kind: model.CurveDSC, Unit: model.UnitCelsius, Points: synthSmokeDSC()}); err != nil {
		t.Fatalf("import dsc: %v", err)
	}
	if _, err := svc.ImportCurve(ImportCurveInput{TrialID: tr.ID, Kind: model.CurveTGA, Unit: model.UnitCelsius, Points: synthSmokeTGA()}); err != nil {
		t.Fatalf("import tga: %v", err)
	}
	if _, err := svc.CreatePrior(CreatePriorInput{FormFrom: "A", FormTo: "B", OnsetLow: 110, OnsetHigh: 130, Direction: model.DirectionEndothermic}); err != nil {
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
		t.Fatalf("events: %v", err)
	}
	split, err := svc.SplitOverlapping(events[0].ID, events[1].ID, "TGA evidence", "A->B", "solvent")
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	if _, err := svc.AdjudicateEvent(split[0].ID, model.EventVetoed, "not supported"); err != nil {
		t.Fatalf("veto: %v", err)
	}
	if _, err := svc.AdjudicateEvent(split[1].ID, model.EventConfirmed, "supported"); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	got, err := svc.GetTrial(tr.ID)
	if err != nil {
		t.Fatalf("get trial: %v", err)
	}
	if got.Status != model.TrialNeedsReview {
		t.Fatalf("status = %s, want needs_review", got.Status)
	}
}
