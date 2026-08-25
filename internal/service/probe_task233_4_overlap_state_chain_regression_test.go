package service

import (
	"testing"

	"task233-thermopoly/internal/model"
)

func TestBug04_OverlapUncertaintyReachesTrialState(t *testing.T) {
	svc := newTestService(t)
	tr, err := svc.CreateTrial(CreateTrialInput{Name: "overlap-chain", Material: "sample", Unit: model.UnitCelsius})
	if err != nil {
		t.Fatalf("create trial: %v", err)
	}
	if _, err := svc.ImportCurve(ImportCurveInput{TrialID: tr.ID, Kind: model.CurveDSC, Unit: model.UnitCelsius, Points: synthSmokeDSC()}); err != nil {
		t.Fatalf("import dsc: %v", err)
	}
	if _, err := svc.ImportCurve(ImportCurveInput{TrialID: tr.ID, Kind: model.CurveTGA, Unit: model.UnitCelsius, Points: synthSmokeTGA()}); err != nil {
		t.Fatalf("import tga: %v", err)
	}
	if _, err := svc.RunBaseline(tr.ID); err != nil {
		t.Fatalf("baseline: %v", err)
	}
	peaks, err := svc.DetectPeaks(tr.ID)
	if err != nil || len(peaks) != 2 {
		t.Fatalf("peaks=%d err=%v, want two peaks", len(peaks), err)
	}
	if !peaks[0].Overlap || !peaks[1].Overlap {
		t.Fatalf("overlap flags=%v/%v, want both true", peaks[0].Overlap, peaks[1].Overlap)
	}
	if _, err := svc.GenerateEvents(tr.ID); err != nil {
		t.Fatalf("generate events: %v", err)
	}
	events, err := svc.ListEvents(tr.ID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 2 || events[0].Status != model.EventOverlapping || events[1].Status != model.EventOverlapping {
		t.Fatalf("event statuses=%v, want both overlapping", []string{events[0].Status, events[1].Status})
	}
	got, err := svc.GetTrial(tr.ID)
	if err != nil {
		t.Fatalf("get trial: %v", err)
	}
	if got.Status != model.TrialNeedsReview {
		t.Fatalf("trial status=%s, want needs_review", got.Status)
	}
}
