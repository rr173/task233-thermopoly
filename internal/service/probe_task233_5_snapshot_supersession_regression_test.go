package service

import (
	"testing"
	"time"

	"task233-thermopoly/internal/model"
)

func seedPublishableSnapshotEvent(t *testing.T, svc *Service) (*model.Trial, string) {
	t.Helper()
	tr, err := svc.CreateTrial(CreateTrialInput{Name: "snapshot-version", Material: "sample", Unit: model.UnitCelsius})
	if err != nil {
		t.Fatalf("create trial: %v", err)
	}
	now := time.Now()
	peak := model.Peak{ID: "pk-snapshot-version", TrialID: tr.ID, CurveID: "curve-snapshot-version", StartIdx: 0, EndIdx: 2, StartTemp: 30, EndTemp: 32, PeakTemp: 31, PeakValue: 1, Direction: model.DirectionEndothermic, Height: 1, Area: 1, Separation: 1, Status: "detected", CreatedAt: now}
	if err := svc.dep.Peaks.InsertMany([]model.Peak{peak}); err != nil {
		t.Fatalf("insert peak: %v", err)
	}
	event := model.Event{ID: "evt-snapshot-version", TrialID: tr.ID, PeakID: peak.ID, Kind: model.EventFusion, Form: "melting", OnsetTemp: 30, PeakTemp: 31, Confidence: 1, Status: model.EventConfirmed, Evidence: "{}", CreatedAt: now, UpdatedAt: now}
	if err := svc.dep.Events.InsertMany([]model.Event{event}); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	return tr, event.ID
}

func TestBug05_PublishingNewSnapshotSupersedesOldVersion(t *testing.T) {
	svc := newTestService(t)
	tr, eventID := seedPublishableSnapshotEvent(t, svc)
	first, err := svc.CreateSnapshot(CreateSnapshotInput{TrialID: tr.ID, Summary: "first", EventIDs: []string{eventID}})
	if err != nil {
		t.Fatalf("create first snapshot: %v", err)
	}
	if _, err := svc.PublishSnapshot(first.ID); err != nil {
		t.Fatalf("publish first snapshot: %v", err)
	}
	second, err := svc.CreateSnapshot(CreateSnapshotInput{TrialID: tr.ID, Summary: "second", EventIDs: []string{eventID}})
	if err != nil {
		t.Fatalf("create second snapshot: %v", err)
	}
	if _, err := svc.PublishSnapshot(second.ID); err != nil {
		t.Fatalf("publish second snapshot: %v", err)
	}
	old, err := svc.GetSnapshot(first.ID)
	if err != nil {
		t.Fatalf("get first snapshot: %v", err)
	}
	newest, err := svc.GetSnapshot(second.ID)
	if err != nil {
		t.Fatalf("get second snapshot: %v", err)
	}
	if old.Status != model.SnapshotSuperseded || old.ReplacedBy != second.ID {
		t.Fatalf("old snapshot=%+v, want superseded by %s", old, second.ID)
	}
	if newest.Status != model.SnapshotPublished {
		t.Fatalf("new snapshot status=%s, want published", newest.Status)
	}
	current, err := svc.dep.Snapshots.Published(tr.ID)
	if err != nil || current.ID != second.ID {
		t.Fatalf("current published snapshot=%v err=%v, want %s", current, err, second.ID)
	}
}
