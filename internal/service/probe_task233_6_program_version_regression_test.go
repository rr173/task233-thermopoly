package service

import (
	"testing"

	"task233-thermopoly/internal/model"
)

func TestBug06_ProgramUpdateKeepsSingleActiveVersion(t *testing.T) {
	svc := newTestService(t)
	tr, err := svc.CreateTrial(CreateTrialInput{Name: "program-version", Material: "sample", Unit: model.UnitCelsius})
	if err != nil {
		t.Fatalf("create trial: %v", err)
	}
	first, err := svc.SetProgram(SetProgramInput{TrialID: tr.ID, Name: "slow", StartTemp: 30, EndTemp: 120, RateKPerMin: 5})
	if err != nil {
		t.Fatalf("set first program: %v", err)
	}
	second, err := svc.SetProgram(SetProgramInput{TrialID: tr.ID, Name: "fast", StartTemp: 30, EndTemp: 150, RateKPerMin: 10})
	if err != nil {
		t.Fatalf("set second program: %v", err)
	}
	if first.Version != 1 || second.Version != 2 {
		t.Fatalf("versions=%d/%d, want 1/2", first.Version, second.Version)
	}
	active, err := svc.GetProgram(tr.ID)
	if err != nil {
		t.Fatalf("get active program: %v", err)
	}
	if active.ID != second.ID || active.Version != 2 || !active.IsActive {
		t.Fatalf("active=%+v, want second version", active)
	}
}
