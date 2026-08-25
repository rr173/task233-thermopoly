package service

import (
	"testing"

	"task233-thermopoly/internal/model"
)

func TestBug03_SealedTrialRejectsEveryMutationPath(t *testing.T) {
	svc := newTestService(t)
	tr, err := svc.CreateTrial(CreateTrialInput{Name: "sealed", Material: "sample", Unit: model.UnitCelsius})
	if err != nil {
		t.Fatalf("create trial: %v", err)
	}
	initial := []model.Point{{Temp: 30, Value: 0}, {Temp: 31, Value: 1}, {Temp: 32, Value: 0}}
	if _, err := svc.ImportCurve(ImportCurveInput{TrialID: tr.ID, Kind: model.CurveDSC, Unit: model.UnitCelsius, Points: initial}); err != nil {
		t.Fatalf("import initial curve: %v", err)
	}
	if _, err := svc.TransitionTrial(tr.ID, model.TrialConfirmed); err != nil {
		t.Fatalf("confirm trial: %v", err)
	}
	sealed, err := svc.SealTrial(tr.ID)
	if err != nil {
		t.Fatalf("seal trial: %v", err)
	}
	if sealed.Status != model.TrialSealed {
		t.Fatalf("sealed status=%s, want sealed", sealed.Status)
	}
	points := []model.Point{{Temp: 40, Value: 0}, {Temp: 41, Value: 2}, {Temp: 42, Value: 0}}
	if _, err := svc.ImportCurve(ImportCurveInput{TrialID: tr.ID, Kind: model.CurveDSC, Unit: model.UnitCelsius, Points: points}); err == nil {
		t.Fatal("sealed trial accepted a new curve")
	}
	if _, err := svc.SetProgram(SetProgramInput{TrialID: tr.ID, Name: "sealed-program", StartTemp: 30, EndTemp: 120, RateKPerMin: 10}); err == nil {
		t.Fatal("sealed trial accepted a heating program")
	}
	if _, err := svc.RunBaseline(tr.ID); err == nil {
		t.Fatal("sealed trial accepted baseline processing")
	}
	curves, err := svc.ListCurves(tr.ID)
	if err != nil {
		t.Fatalf("list curves: %v", err)
	}
	if len(curves) != 1 {
		t.Fatalf("sealed trial stored %d curves, want only initial curve", len(curves))
	}
	if _, err := svc.GetProgram(tr.ID); err == nil {
		t.Fatal("sealed trial stored a new program")
	}
}
