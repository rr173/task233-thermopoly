package service

import (
	"sync"
	"testing"

	"task233-thermopoly/internal/model"
)

func TestBug02_ConcurrentDuplicateCurveImportIsSerialized(t *testing.T) {
	svc := newTestService(t)
	tr, err := svc.CreateTrial(CreateTrialInput{Name: "concurrent-curves", Material: "sample", Unit: model.UnitCelsius})
	if err != nil {
		t.Fatalf("create trial: %v", err)
	}
	points := []model.Point{{Temp: 30, Value: 0}, {Temp: 31, Value: 1}, {Temp: 32, Value: 0}}
	const workers = 20
	start := make(chan struct{})
	results := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := svc.ImportCurve(ImportCurveInput{TrialID: tr.ID, Kind: model.CurveDSC, Unit: model.UnitCelsius, Points: points})
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	successes := 0
	duplicates := 0
	for err := range results {
		if err == nil {
			successes++
		} else if model.IsKind(err, model.ErrCurveDuplicate) {
			duplicates++
		}
	}
	if successes != 1 || duplicates != workers-1 {
		t.Fatalf("concurrent import results: successes=%d duplicates=%d", successes, duplicates)
	}
	curves, err := svc.ListCurves(tr.ID)
	if err != nil {
		t.Fatalf("list curves: %v", err)
	}
	if len(curves) != 1 {
		t.Fatalf("stored curves=%d, want 1", len(curves))
	}
}
