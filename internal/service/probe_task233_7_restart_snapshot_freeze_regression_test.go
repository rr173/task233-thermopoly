package service

import (
	"path/filepath"
	"testing"
	"time"

	"task233-thermopoly/internal/model"
	"task233-thermopoly/internal/store"
)

func TestBug07_PublishedSnapshotFreezeSurvivesRestart(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "thermopoly.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open first store: %v", err)
	}
	svc := New(Deps{Trials: store.NewTrialStore(st.DB()), Curves: store.NewCurveStore(st.DB()), Programs: store.NewProgramStore(st.DB()), Segments: store.NewSegmentStore(st.DB()), Peaks: store.NewPeakStore(st.DB()), Events: store.NewEventStore(st.DB()), Snapshots: store.NewSnapshotStore(st.DB()), Priors: store.NewPriorStore(st.DB())})
	tr, err := svc.CreateTrial(CreateTrialInput{Name: "restart-freeze", Material: "sample", Unit: model.UnitCelsius})
	if err != nil {
		t.Fatalf("create trial: %v", err)
	}
	curve, err := svc.ImportCurve(ImportCurveInput{TrialID: tr.ID, Kind: model.CurveDSC, Unit: model.UnitCelsius, Points: []model.Point{{Temp: 30, Value: 0}, {Temp: 31, Value: 1}, {Temp: 32, Value: 0}}})
	if err != nil {
		t.Fatalf("import curve: %v", err)
	}
	now := time.Now()
	peak := model.Peak{ID: "pk-restart-freeze", TrialID: tr.ID, CurveID: curve.ID, StartIdx: 0, EndIdx: 2, StartTemp: 30, EndTemp: 32, PeakTemp: 31, PeakValue: 1, Direction: model.DirectionEndothermic, Height: 1, Area: 1, Separation: 1, Status: "detected", CreatedAt: now}
	if err := svc.dep.Peaks.InsertMany([]model.Peak{peak}); err != nil {
		t.Fatalf("insert peak: %v", err)
	}
	event := model.Event{ID: "evt-restart-freeze", TrialID: tr.ID, PeakID: peak.ID, Kind: model.EventFusion, Form: "melting", OnsetTemp: 30, PeakTemp: 31, Confidence: 1, Status: model.EventConfirmed, Evidence: "{}", CreatedAt: now, UpdatedAt: now}
	if err := svc.dep.Events.InsertMany([]model.Event{event}); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	sn, err := svc.CreateSnapshot(CreateSnapshotInput{TrialID: tr.ID, Summary: "restart", EventIDs: []string{event.ID}})
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	if _, err := svc.PublishSnapshot(sn.ID); err != nil {
		t.Fatalf("publish snapshot: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("close first store: %v", err)
	}

	st, err = store.Open(dbPath)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer st.Close()
	restarted := New(Deps{Trials: store.NewTrialStore(st.DB()), Curves: store.NewCurveStore(st.DB()), Programs: store.NewProgramStore(st.DB()), Segments: store.NewSegmentStore(st.DB()), Peaks: store.NewPeakStore(st.DB()), Events: store.NewEventStore(st.DB()), Snapshots: store.NewSnapshotStore(st.DB()), Priors: store.NewPriorStore(st.DB())})
	if err := restarted.VerifySnapshotInput(sn.ID); err != nil {
		t.Fatalf("verify after restart: %v", err)
	}
}
