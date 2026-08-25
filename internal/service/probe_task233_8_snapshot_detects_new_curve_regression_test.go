package service

import (
	"testing"
	"time"

	"task233-thermopoly/internal/model"
)

func seedSnapshotFreezeMutationEvent(t *testing.T, svc *Service) (*model.Trial, string) {
	t.Helper()
	tr, err := svc.CreateTrial(CreateTrialInput{Name: "freeze-mutation", Material: "sample", Unit: model.UnitCelsius})
	if err != nil {
		t.Fatalf("create trial: %v", err)
	}
	now := time.Now()
	peak := model.Peak{ID: "pk-freeze-mutation", TrialID: tr.ID, CurveID: "curve-freeze-mutation", StartIdx: 0, EndIdx: 2, StartTemp: 30, EndTemp: 32, PeakTemp: 31, PeakValue: 1, Direction: model.DirectionEndothermic, Height: 1, Area: 1, Separation: 1, Status: "detected", CreatedAt: now}
	if err := svc.dep.Peaks.InsertMany([]model.Peak{peak}); err != nil {
		t.Fatalf("insert peak: %v", err)
	}
	event := model.Event{ID: "evt-freeze-mutation", TrialID: tr.ID, PeakID: peak.ID, Kind: model.EventFusion, Form: "melting", OnsetTemp: 30, PeakTemp: 31, Confidence: 1, Status: model.EventConfirmed, Evidence: "{}", CreatedAt: now, UpdatedAt: now}
	if err := svc.dep.Events.InsertMany([]model.Event{event}); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	return tr, event.ID
}

func TestBug08_PublishedSnapshotDetectsLaterCurveImport(t *testing.T) {
	svc := newTestService(t)
	tr, eventID := seedSnapshotFreezeMutationEvent(t, svc)
	if _, err := svc.ImportCurve(ImportCurveInput{TrialID: tr.ID, Kind: model.CurveDSC, Unit: model.UnitCelsius, Points: []model.Point{{Temp: 30, Value: 0}, {Temp: 31, Value: 1}, {Temp: 32, Value: 0}}}); err != nil {
		t.Fatalf("import initial curve: %v", err)
	}
	sn, err := svc.CreateSnapshot(CreateSnapshotInput{TrialID: tr.ID, Summary: "freeze", EventIDs: []string{eventID}})
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	if _, err := svc.PublishSnapshot(sn.ID); err != nil {
		t.Fatalf("publish snapshot: %v", err)
	}
	if _, err := svc.ImportCurve(ImportCurveInput{TrialID: tr.ID, Kind: model.CurveTGA, Unit: model.UnitCelsius, Points: []model.Point{{Temp: 30, Value: 100}, {Temp: 31, Value: 99}, {Temp: 32, Value: 99}}}); err != nil {
		t.Fatalf("import later curve: %v", err)
	}
	if err := svc.VerifySnapshotInput(sn.ID); err == nil {
		t.Fatal("published snapshot accepted a later curve without reporting input conflict")
	}
}
