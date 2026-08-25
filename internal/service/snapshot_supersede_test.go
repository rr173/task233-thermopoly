package service

import (
	"testing"

	"task233-thermopoly/internal/model"
)

// buildConfirmedTrial 构造一个已完成基线/峰/事件裁决、试验状态为
// confirmed 的试验，便于快照相关测试复用。
func buildConfirmedTrial(t *testing.T, svc *Service) string {
	t.Helper()
	tr, err := svc.CreateTrial(CreateTrialInput{Name: "snap", Material: "FormA", Unit: model.UnitCelsius})
	if err != nil {
		t.Fatalf("create trial: %v", err)
	}
	tid := tr.ID
	if _, err := svc.ImportCurve(ImportCurveInput{TrialID: tid, Kind: model.CurveDSC, Unit: model.UnitCelsius, Points: synthSmokeDSC()}); err != nil {
		t.Fatalf("import dsc: %v", err)
	}
	if _, err := svc.ImportCurve(ImportCurveInput{TrialID: tid, Kind: model.CurveTGA, Unit: model.UnitCelsius, Points: synthSmokeTGA()}); err != nil {
		t.Fatalf("import tga: %v", err)
	}
	if _, err := svc.CreatePrior(CreatePriorInput{
		FormFrom: "FormA", FormTo: "FormB", OnsetLow: 110, OnsetHigh: 130,
		Direction: model.DirectionEndothermic, MaxMassLossPct: 0.5,
	}); err != nil {
		t.Fatalf("create prior: %v", err)
	}
	if _, err := svc.RunBaseline(tid); err != nil {
		t.Fatalf("baseline: %v", err)
	}
	if _, err := svc.DetectPeaks(tid); err != nil {
		t.Fatalf("peaks: %v", err)
	}
	events, err := svc.GenerateEvents(tid)
	if err != nil {
		t.Fatalf("events: %v", err)
	}
	split, err := svc.SplitOverlapping(events[0].ID, events[1].ID, "TGA evidence", "FormA->FormB", "solvent")
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	for _, e := range split {
		if _, err := svc.AdjudicateEvent(e.ID, model.EventConfirmed, "ok"); err != nil {
			t.Fatalf("adjudicate: %v", err)
		}
	}
	return tid
}

// TestPublishNewSnapshotSupersedesOld 验证：为同一试验发布新的判读快照后，
// 旧版本应明确变为 superseded，新版本成为当前发布版本。
// 回归：旧代码的 Supersede 为空操作、service 层替代调用缺失，导致旧版本
// 仍以 published 状态留存，被误当作已发布结果。
func TestPublishNewSnapshotSupersedesOld(t *testing.T) {
	svc := newTestService(t)
	tid := buildConfirmedTrial(t, svc)

	events, _ := svc.ListEvents(tid)
	ids := []string{events[0].ID, events[1].ID}

	// 发布 v1
	v1, err := svc.CreateSnapshot(CreateSnapshotInput{TrialID: tid, Summary: "v1", EventIDs: ids})
	if err != nil {
		t.Fatalf("create v1: %v", err)
	}
	if _, err := svc.PublishSnapshot(v1.ID); err != nil {
		t.Fatalf("publish v1: %v", err)
	}

	// 发布 v2：应替代 v1
	v2, err := svc.CreateSnapshot(CreateSnapshotInput{TrialID: tid, Summary: "v2", EventIDs: ids})
	if err != nil {
		t.Fatalf("create v2: %v", err)
	}
	if _, err := svc.PublishSnapshot(v2.ID); err != nil {
		t.Fatalf("publish v2: %v", err)
	}

	// v2 为当前发布版本
	cur, err := svc.dep.Snapshots.Published(tid)
	if err != nil {
		t.Fatalf("current published: %v", err)
	}
	if cur.ID != v2.ID {
		t.Fatalf("current published = %s, want %s (v2)", cur.ID, v2.ID)
	}
	if cur.Status != model.SnapshotPublished {
		t.Fatalf("v2 status = %s, want published", cur.Status)
	}

	// v1 已被明确替代
	got, err := svc.GetSnapshot(v1.ID)
	if err != nil {
		t.Fatalf("get v1: %v", err)
	}
	if got.Status != model.SnapshotSuperseded {
		t.Errorf("v1 status = %s, want superseded", got.Status)
	}
	if got.ReplacedBy != v2.ID {
		t.Errorf("v1 replaced_by = %q, want %q", got.ReplacedBy, v2.ID)
	}

	// 列表应包含两个快照且版本号递增
	all, err := svc.ListSnapshots(tid)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(all))
	}
	published, superseded := 0, 0
	for _, sn := range all {
		switch sn.Status {
		case model.SnapshotPublished:
			published++
		case model.SnapshotSuperseded:
			superseded++
		}
	}
	if published != 1 || superseded != 1 {
		t.Fatalf("expected 1 published + 1 superseded, got %d published + %d superseded", published, superseded)
	}
}

// TestSupersedeRejectsNonPublished 确认 Supersede 仅作用于 published 快照，
// 对 draft/superseded 状态拒绝替代（状态机约束）。
func TestSupersedeRejectsNonPublished(t *testing.T) {
	svc := newTestService(t)
	_ = svc.snapSvc // 仅复用 newTestService 搭建的依赖；直接测领域服务逻辑

	draft := model.Snapshot{ID: "snp-draft", Status: model.SnapshotDraft}
	if err := svc.snapSvc.Supersede(&draft, "new"); err == nil {
		t.Fatal("superseding a draft must fail")
	}
	sup := model.Snapshot{ID: "snp-sup", Status: model.SnapshotSuperseded}
	if err := svc.snapSvc.Supersede(&sup, "new"); err == nil {
		t.Fatal("superseding an already-superseded snapshot must fail")
	}
}
