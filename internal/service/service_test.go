package service

import (
	"math"
	"testing"

	"task233-thermopoly/internal/model"
	"task233-thermopoly/internal/store"
)

// newTestService 构造内存库 Service。
func newTestService(t *testing.T) *Service {
	t.Helper()
	st, err := store.Open("")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return New(Deps{
		Trials:    store.NewTrialStore(st.DB()),
		Curves:    store.NewCurveStore(st.DB()),
		Programs:  store.NewProgramStore(st.DB()),
		Segments:  store.NewSegmentStore(st.DB()),
		Peaks:     store.NewPeakStore(st.DB()),
		Events:    store.NewEventStore(st.DB()),
		Snapshots: store.NewSnapshotStore(st.DB()),
		Priors:    store.NewPriorStore(st.DB()),
	})
}

func TestCreateAndGetTrial(t *testing.T) {
	svc := newTestService(t)
	tr, err := svc.CreateTrial(CreateTrialInput{Name: "t1", Material: "m1", Unit: model.UnitCelsius})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if tr.Status != model.TrialReceiving {
		t.Errorf("status = %s", tr.Status)
	}
	got, err := svc.GetTrial(tr.ID)
	if err != nil || got.ID != tr.ID {
		t.Fatalf("get: %v", err)
	}
	// 缺材料应报错
	if _, err := svc.CreateTrial(CreateTrialInput{Name: "t2"}); err == nil {
		t.Error("trial without material must fail")
	}
}

func TestTransitionRules(t *testing.T) {
	svc := newTestService(t)
	tr, _ := svc.CreateTrial(CreateTrialInput{Name: "t", Material: "m"})
	// receiving -> sealed 不允许（必须先 confirmed）
	if _, err := svc.SealTrial(tr.ID); err == nil {
		t.Error("sealing unconfirmed trial must fail")
	}
	// receiving -> needs_review 允许
	got, err := svc.TransitionTrial(tr.ID, model.TrialNeedsReview)
	if err != nil {
		t.Fatalf("transition: %v", err)
	}
	if got.Status != model.TrialNeedsReview {
		t.Errorf("status = %s", got.Status)
	}
	// 回退不允许
	if _, err := svc.TransitionTrial(tr.ID, model.TrialReceiving); err == nil {
		t.Error("backward transition must fail")
	}
}

func TestDuplicateCurveRejected(t *testing.T) {
	svc := newTestService(t)
	tr, _ := svc.CreateTrial(CreateTrialInput{Name: "t", Material: "m", Unit: model.UnitCelsius})
	pts := synthSmokeDSC()
	if _, err := svc.ImportCurve(ImportCurveInput{
		TrialID: tr.ID, Kind: model.CurveDSC, Name: "a", Unit: model.UnitCelsius, Points: pts,
	}); err != nil {
		t.Fatalf("import: %v", err)
	}
	if _, err := svc.ImportCurve(ImportCurveInput{
		TrialID: tr.ID, Kind: model.CurveDSC, Name: "b", Unit: model.UnitCelsius, Points: pts,
	}); err == nil {
		t.Fatal("duplicate curve content must be rejected (hash idempotency)")
	}
}

func TestMixedUnitsRejected(t *testing.T) {
	svc := newTestService(t)
	tr, _ := svc.CreateTrial(CreateTrialInput{Name: "t", Material: "m", Unit: model.UnitCelsius})
	if _, err := svc.ImportCurve(ImportCurveInput{
		TrialID: tr.ID, Kind: model.CurveDSC, Unit: model.UnitKelvin, Points: synthSmokeDSC(),
	}); err == nil {
		t.Fatal("mixed temperature units must be rejected")
	}
}

func TestEndToEndAnalysis(t *testing.T) {
	svc := newTestService(t)
	tr, _ := svc.CreateTrial(CreateTrialInput{Name: "t", Material: "m", Unit: model.UnitCelsius})
	tid := tr.ID
	if _, err := svc.ImportCurve(ImportCurveInput{TrialID: tid, Kind: model.CurveDSC, Unit: model.UnitCelsius, Points: synthSmokeDSC()}); err != nil {
		t.Fatalf("dsc: %v", err)
	}
	if _, err := svc.ImportCurve(ImportCurveInput{TrialID: tid, Kind: model.CurveTGA, Unit: model.UnitCelsius, Points: synthSmokeTGA()}); err != nil {
		t.Fatalf("tga: %v", err)
	}
	if _, err := svc.CreatePrior(CreatePriorInput{
		FormFrom: "FormA", FormTo: "FormB", OnsetLow: 110, OnsetHigh: 130,
		Direction: model.DirectionEndothermic, MaxMassLossPct: 0.5,
	}); err != nil {
		t.Fatalf("prior: %v", err)
	}
	if _, err := svc.RunBaseline(tid); err != nil {
		t.Fatalf("baseline: %v", err)
	}
	peaks, err := svc.DetectPeaks(tid)
	if err != nil {
		t.Fatalf("peaks: %v", err)
	}
	if len(peaks) != 2 {
		t.Fatalf("expected 2 peaks, got %d", len(peaks))
	}
	events, err := svc.GenerateEvents(tid)
	if err != nil {
		t.Fatalf("events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	tr, _ = svc.GetTrial(tid)
	if tr.Status != model.TrialNeedsReview {
		t.Errorf("trial status = %s, want needs_review (overlap)", tr.Status)
	}
	// 拆分 + 裁决 + 发布
	split, err := svc.SplitOverlapping(events[0].ID, events[1].ID,
		"TGA derivative 2.5% loss at 126C; DSC shoulder at 120C", "FormA->FormB", "solvent")
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	for _, e := range split {
		if _, err := svc.AdjudicateEvent(e.ID, model.EventConfirmed, "ok"); err != nil {
			t.Fatalf("adjudicate: %v", err)
		}
	}
	sn, err := svc.CreateSnapshot(CreateSnapshotInput{
		TrialID:  tid,
		Summary:  "s",
		EventIDs: []string{split[0].ID, split[1].ID},
	})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	pub, err := svc.PublishSnapshot(sn.ID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if pub.Status != model.SnapshotPublished {
		t.Errorf("snapshot status = %s", pub.Status)
	}
	tr, _ = svc.GetTrial(tid)
	if tr.Status != model.TrialConfirmed {
		t.Errorf("trial status = %s, want confirmed", tr.Status)
	}
	// 封存
	if _, err := svc.SealTrial(tid); err != nil {
		t.Fatalf("seal: %v", err)
	}
	// 封存后修改拒绝
	if _, err := svc.ImportCurve(ImportCurveInput{TrialID: tid, Kind: model.CurveDSC, Unit: model.UnitCelsius, Points: synthSmokeDSC()}); err == nil {
		t.Fatal("sealed trial must reject curve import")
	}
	// 快照输入冻结校验
	if err := svc.VerifySnapshotInput(sn.ID); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

// synthSmokeDSC 合成与 smoke-test 相同的双重叠峰 DSC 曲线。
func synthSmokeDSC() []model.Point {
	var pts []model.Point
	for temp := 30.0; temp <= 200.0; temp += 1.0 {
		d1 := (temp - 120) / 3.0
		d2 := (temp - 126) / 3.0
		v := 1.0*exp(-d1*d1) + 0.8*exp(-d2*d2)
		pts = append(pts, model.Point{Temp: temp, Value: v})
	}
	return pts
}

// synthSmokeTGA 合成与 smoke-test 相同的阶梯失重 TGA 曲线。
func synthSmokeTGA() []model.Point {
	var pts []model.Point
	for temp := 30.0; temp <= 200.0; temp += 1.0 {
		mass := 100.0
		if temp > 124 && temp <= 127 {
			mass = 100.0 - 5.0*(temp-124)/3.0
		} else if temp > 127 {
			mass = 95.0
		}
		pts = append(pts, model.Point{Temp: temp, Value: mass})
	}
	return pts
}

func exp(v float64) float64 {
	return math.Exp(v)
}
