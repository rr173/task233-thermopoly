package event

import (
	"testing"

	"task233-thermopoly/internal/model"
)

func TestPriorRequiresTGAEvidence(t *testing.T) {
	classifier := NewClassifier([]model.PolymorphPrior{{
		ID: "prior-1", FormFrom: "A", FormTo: "B", OnsetLow: 120, OnsetHigh: 130,
		Direction: model.DirectionEndothermic, MaxMassLossPct: 0.5, Active: true,
	}})
	candidate, err := classifier.Classify(ClassifyInput{
		Peak:   model.Peak{ID: "peak-1", PeakTemp: 125, StartTemp: 120, Direction: model.DirectionEndothermic},
		HasTGA: false,
	})
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if candidate.Event.Kind == model.EventPolymorph {
		t.Fatalf("missing TGA evidence must not produce polymorph event: %+v", candidate.Event)
	}
}
