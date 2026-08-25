package service

import (
	"sync"
	"testing"
	"time"

	"task233-thermopoly/internal/model"
)

func TestBug10_ConcurrentSnapshotDraftsKeepUniqueVersions(t *testing.T) {
	svc := newTestService(t)
	tr, err := svc.CreateTrial(CreateTrialInput{Name: "concurrent-snapshots", Material: "sample", Unit: model.UnitCelsius})
	if err != nil {
		t.Fatalf("create trial: %v", err)
	}
	now := time.Now()
	peak := model.Peak{ID: "pk-concurrent-snapshots", TrialID: tr.ID, CurveID: "curve-concurrent-snapshots", StartIdx: 0, EndIdx: 2, StartTemp: 30, EndTemp: 32, PeakTemp: 31, PeakValue: 1, Direction: model.DirectionEndothermic, Height: 1, Area: 1, Separation: 1, Status: "detected", CreatedAt: now}
	if err := svc.dep.Peaks.InsertMany([]model.Peak{peak}); err != nil {
		t.Fatalf("insert peak: %v", err)
	}
	event := model.Event{ID: "evt-concurrent-snapshots", TrialID: tr.ID, PeakID: peak.ID, Kind: model.EventFusion, Form: "melting", OnsetTemp: 30, PeakTemp: 31, Confidence: 1, Status: model.EventConfirmed, Evidence: "{}", CreatedAt: now, UpdatedAt: now}
	if err := svc.dep.Events.InsertMany([]model.Event{event}); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	const workers = 20
	start := make(chan struct{})
	results := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := svc.CreateSnapshot(CreateSnapshotInput{TrialID: tr.ID, Summary: "concurrent", EventIDs: []string{event.ID}})
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatalf("concurrent snapshot draft failed: %v", err)
		}
	}
	snapshots, err := svc.ListSnapshots(tr.ID)
	if err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	if len(snapshots) != workers {
		t.Fatalf("snapshot count=%d, want %d", len(snapshots), workers)
	}
}
