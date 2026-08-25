package event

import (
	"testing"

	"task233-thermopoly/internal/model"
)

func TestClassifyPolymorphMatch(t *testing.T) {
	priors := []model.PolymorphPrior{
		{ID: "p1", FormFrom: "FormA", FormTo: "FormB", OnsetLow: 110, OnsetHigh: 130,
			Direction: model.DirectionEndothermic, MaxMassLossPct: 0.5, Active: true},
	}
	cl := NewClassifier(priors)
	cand, err := cl.Classify(ClassifyInput{
		Peak:        model.Peak{PeakTemp: 120, Direction: model.DirectionEndothermic},
		MassLossPct: 0.1,
		HasTGA:      true,
	})
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if cand.Event.Kind != model.EventPolymorph {
		t.Errorf("kind = %s, want polymorph", cand.Event.Kind)
	}
	if cand.Event.Form != "FormA->FormB" {
		t.Errorf("form = %s", cand.Event.Form)
	}
	if cand.Event.Confidence < 0.8 {
		t.Errorf("confidence = %v, want >= 0.8", cand.Event.Confidence)
	}
}

func TestClassifyDesolvationByMassLoss(t *testing.T) {
	cl := NewClassifier(nil) // 无先验
	cand, err := cl.Classify(ClassifyInput{
		Peak:        model.Peak{PeakTemp: 126, Direction: model.DirectionEndothermic},
		MassLossPct: 2.5,
		HasTGA:      true,
	})
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if cand.Event.Kind != model.EventDesolvation {
		t.Errorf("kind = %s, want desolvation", cand.Event.Kind)
	}
}

func TestClassifyFusionWithoutMassLoss(t *testing.T) {
	cl := NewClassifier(nil)
	cand, err := cl.Classify(ClassifyInput{
		Peak:        model.Peak{PeakTemp: 170, Direction: model.DirectionEndothermic},
		MassLossPct: 0,
		HasTGA:      true,
	})
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if cand.Event.Kind != model.EventFusion {
		t.Errorf("kind = %s, want fusion", cand.Event.Kind)
	}
}

func TestAdjudicateFlow(t *testing.T) {
	a := NewAdjudicator()
	e := &model.Event{ID: "e1", Status: model.EventCandidate}
	if err := a.Adjudicate(e, model.EventVetoed, "bad peak"); err != nil {
		t.Fatalf("veto: %v", err)
	}
	if e.Status != model.EventVetoed {
		t.Errorf("status = %s", e.Status)
	}
	// 否决态不能再改
	if err := a.Adjudicate(e, model.EventConfirmed, ""); err == nil {
		t.Error("vetoed event must not be confirmed")
	}
}

func TestAdjudicateRejectsOverlapConfirm(t *testing.T) {
	a := NewAdjudicator()
	e := &model.Event{ID: "e1", Status: model.EventOverlapping}
	if err := a.Adjudicate(e, model.EventConfirmed, ""); err == nil {
		t.Error("overlapping event must not be confirmed without split")
	}
}

func TestSplitRequiresEvidence(t *testing.T) {
	a := NewAdjudicator()
	_, _, err := a.Split(SplitInput{
		EventA:   model.Event{ID: "a", Status: model.EventOverlapping},
		EventB:   model.Event{ID: "b", Status: model.EventOverlapping},
		Evidence: "x",
	})
	if err == nil {
		t.Error("split with trivial evidence must fail")
	}
	ea, eb, err := a.Split(SplitInput{
		EventA:   model.Event{ID: "a", Status: model.EventOverlapping},
		EventB:   model.Event{ID: "b", Status: model.EventOverlapping},
		Evidence: "TGA derivative shows 2.5% loss at 126C",
		FormA:    "FormA->FormB",
		FormB:    "solvent",
	})
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	if ea.Status != model.EventCandidate || eb.Status != model.EventCandidate {
		t.Error("split must yield candidates")
	}
}
