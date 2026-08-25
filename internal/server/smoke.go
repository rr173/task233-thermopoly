package server

import (
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"

	"task233-thermopoly/internal/model"
	"task233-thermopoly/internal/service"
)

// SmokeTest 执行端到端自检（--smoke-test 模式，Docker 双架构验证的唯一判据）：
// 1. 用临时 DB 完整跑一遍业务闭环（建试验->导曲线->基线->峰->事件->拆分->裁决->快照发布）；
// 2. 关闭并重新打开同一 DB，验证持久化与重启恢复（试验/峰/事件/快照仍在）；
// 3. 校验已发布快照输入冻结性。
// 全程任一步失败返回非 nil 错误，进程以非 0 退出。
func SmokeTest(dbPath string) error {
	if dbPath == "" {
		dbPath = filepath.Join(os.TempDir(), fmt.Sprintf("thermopoly-smoke-%d.db", os.Getpid()))
	}
	log.Printf("smoke-test: db=%s", dbPath)

	// --- 第一轮：写入 ---
	s1, err := New(Config{Addr: ":0", DB: dbPath})
	if err != nil {
		return fmt.Errorf("open db (round 1): %w", err)
	}
	svc := s1.Service()

	trial, err := svc.CreateTrial(service.CreateTrialInput{
		Name:     "Smoke DSC/TGA trial",
		Material: "API compound X",
		BatchNo:  "SMOKE-001",
		Unit:     model.UnitCelsius,
	})
	if err != nil {
		return fmt.Errorf("create trial: %w", err)
	}
	tid := trial.ID
	log.Printf("smoke-test: trial created %s (unit=%s)", tid, trial.Unit)

	if _, err := svc.SetProgram(service.SetProgramInput{
		TrialID: tid, Name: "10K/min ramp", StartTemp: 30, EndTemp: 200, RateKPerMin: 10,
	}); err != nil {
		return fmt.Errorf("set program: %w", err)
	}

	// 合成 DSC 曲线：两个重叠吸热峰（120°C / 126°C）
	dsc := synthDSC()
	if _, err := svc.ImportCurve(service.ImportCurveInput{
		TrialID: tid, Kind: model.CurveDSC, Name: "DSC heat flow", Unit: model.UnitCelsius, Points: dsc,
	}); err != nil {
		return fmt.Errorf("import dsc: %w", err)
	}
	// 合成 TGA 曲线：124-127°C 快速失重 5%（溶剂脱除证据）
	tga := synthTGA()
	if _, err := svc.ImportCurve(service.ImportCurveInput{
		TrialID: tid, Kind: model.CurveTGA, Name: "TGA mass", Unit: model.UnitCelsius, Points: tga,
	}); err != nil {
		return fmt.Errorf("import tga: %w", err)
	}

	// 晶型先验：Form A->B 转变，110-130°C 吸热、几乎无质量损失
	prior, err := svc.CreatePrior(service.CreatePriorInput{
		FormFrom: "FormA", FormTo: "FormB",
		OnsetLow: 110, OnsetHigh: 130,
		Direction:      model.DirectionEndothermic,
		MaxMassLossPct: 0.5,
		Note:           "reference DSC evidence",
	})
	if err != nil {
		return fmt.Errorf("create prior: %w", err)
	}
	log.Printf("smoke-test: prior created %s", prior.ID)

	// 基线校正
	if _, err := svc.RunBaseline(tid); err != nil {
		return fmt.Errorf("baseline: %w", err)
	}
	// 峰检测：应检出 2 个峰且标记重叠
	peaks, err := svc.DetectPeaks(tid)
	if err != nil {
		return fmt.Errorf("detect peaks: %w", err)
	}
	if len(peaks) != 2 {
		return fmt.Errorf("expected 2 peaks, got %d", len(peaks))
	}
	overlapCount := 0
	for _, p := range peaks {
		if p.Overlap {
			overlapCount++
		}
	}
	if overlapCount < 2 {
		return fmt.Errorf("expected overlapping peaks, got %d/%d (sep=%v)", overlapCount, len(peaks), peaks[0].Separation)
	}
	log.Printf("smoke-test: %d peaks detected, overlapping=%d", len(peaks), overlapCount)

	// 事件生成：重叠峰 -> overlapping 事件，试验进入 needs_review
	events, err := svc.GenerateEvents(tid)
	if err != nil {
		return fmt.Errorf("generate events: %w", err)
	}
	if len(events) != 2 {
		return fmt.Errorf("expected 2 events, got %d", len(events))
	}
	evA, evB := events[0], events[1]
	if evA.Status != model.EventOverlapping || evB.Status != model.EventOverlapping {
		return fmt.Errorf("expected both events overlapping, got %s/%s", evA.Status, evB.Status)
	}
	tr, _ := svc.GetTrial(tid)
	if tr.Status != model.TrialNeedsReview {
		return fmt.Errorf("expected trial needs_review, got %s", tr.Status)
	}
	log.Printf("smoke-test: events overlapping -> trial %s", tr.Status)

	// 补充 TGA 证据拆分重叠：峰1=FormA->FormB，峰2=溶剂脱除
	split, err := svc.SplitOverlapping(evA.ID, evB.ID,
		"TGA derivative shows 2.5% mass loss at 126C matching solvent; DSC shoulder at 120C matches FormA->FormB",
		"FormA->FormB", "solvent/water removal")
	if err != nil {
		return fmt.Errorf("split overlap: %w", err)
	}
	if len(split) != 2 || split[0].Status != model.EventCandidate || split[1].Status != model.EventCandidate {
		return fmt.Errorf("split did not yield two candidates")
	}
	log.Printf("smoke-test: overlap split -> %s / %s", split[0].Kind, split[1].Kind)

	// 裁决全部事件为 confirmed
	for _, e := range split {
		cur, err := svc.AdjudicateEvent(e.ID, model.EventConfirmed, "smoke adjudication")
		if err != nil {
			return fmt.Errorf("adjudicate %s: %w", e.ID, err)
		}
		log.Printf("smoke-test: event %s -> %s (%s)", cur.ID, cur.Status, cur.Form)
	}

	// 创建并发布快照
	sn, err := svc.CreateSnapshot(service.CreateSnapshotInput{
		TrialID:  tid,
		Summary:  "smoke published judgement",
		EventIDs: []string{split[0].ID, split[1].ID},
	})
	if err != nil {
		return fmt.Errorf("create snapshot: %w", err)
	}
	published, err := svc.PublishSnapshot(sn.ID)
	if err != nil {
		return fmt.Errorf("publish snapshot: %w", err)
	}
	if published.Status != model.SnapshotPublished {
		return fmt.Errorf("expected published snapshot, got %s", published.Status)
	}
	tr, _ = svc.GetTrial(tid)
	if tr.Status != model.TrialConfirmed {
		return fmt.Errorf("expected trial confirmed after publish, got %s", tr.Status)
	}
	log.Printf("smoke-test: snapshot v%d published (trial=%s)", published.Version, tr.Status)

	// 封存试验
	sealed, err := svc.SealTrial(tid)
	if err != nil {
		return fmt.Errorf("seal trial: %w", err)
	}
	log.Printf("smoke-test: trial sealed at %v", sealed.SealedAt)

	// --- 重启恢复：关闭并重开同一 DB ---
	if err := s1.Shutdown(nil); err != nil {
		return fmt.Errorf("close db (round 1): %w", err)
	}
	log.Printf("smoke-test: db closed, reopening for recovery check")

	s2, err := New(Config{Addr: ":0", DB: dbPath})
	if err != nil {
		return fmt.Errorf("reopen db (round 2): %w", err)
	}
	defer s2.Shutdown(nil)

	recovered, err := s2.Service().GetTrial(tid)
	if err != nil {
		return fmt.Errorf("recover trial: %w", err)
	}
	if recovered.Status != model.TrialSealed {
		return fmt.Errorf("recovered trial status %s != sealed", recovered.Status)
	}
	recoveredPeaks, err := s2.Service().ListPeaks(tid)
	if err != nil {
		return fmt.Errorf("recover peaks: %w", err)
	}
	recoveredEvents, err := s2.Service().ListEvents(tid)
	if err != nil {
		return fmt.Errorf("recover events: %w", err)
	}
	recoveredSnaps, err := s2.Service().ListSnapshots(tid)
	if err != nil {
		return fmt.Errorf("recover snapshots: %w", err)
	}
	if len(recoveredPeaks) != 2 || len(recoveredEvents) != 2 || len(recoveredSnaps) != 1 {
		return fmt.Errorf("recovery mismatch: peaks=%d events=%d snaps=%d",
			len(recoveredPeaks), len(recoveredEvents), len(recoveredSnaps))
	}
	log.Printf("smoke-test: recovery ok (peaks=%d events=%d snapshots=%d)",
		len(recoveredPeaks), len(recoveredEvents), len(recoveredSnaps))

	// 快照输入冻结性校验
	if err := s2.Service().VerifySnapshotInput(recoveredSnaps[0].ID); err != nil {
		return fmt.Errorf("verify frozen inputs: %w", err)
	}

	// 幂等：重复导入同一条 DSC 曲线应被拒绝
	if _, err := s2.Service().ImportCurve(service.ImportCurveInput{
		TrialID: tid, Kind: model.CurveDSC, Name: "dup", Unit: model.UnitCelsius, Points: dsc,
	}); err == nil {
		return fmt.Errorf("duplicate curve import should have failed on sealed trial")
	}
	log.Printf("smoke-test: sealed trial rejects modification (idempotency/immutability ok)")

	fmt.Println("SMOKE TEST PASSED")
	return nil
}

// synthDSC 合成 DSC 热流曲线：基线 0，两个高斯吸热峰。
func synthDSC() []model.Point {
	peaks := []struct {
		t, a, s float64
	}{
		{120, 1.0, 3.0}, // Form A->B 转变
		{126, 0.8, 3.0}, // 溶剂脱除（与峰 1 重叠）
	}
	var pts []model.Point
	for temp := 30.0; temp <= 200.0; temp += 1.0 {
		v := 0.0
		for _, p := range peaks {
			d := (temp - p.t) / p.s
			v += p.a * math.Exp(-d*d)
		}
		pts = append(pts, model.Point{Temp: temp, Value: v})
	}
	return pts
}

// synthTGA 合成 TGA 质量曲线：124-127°C 快速失重 5%。
func synthTGA() []model.Point {
	var pts []model.Point
	for temp := 30.0; temp <= 200.0; temp += 1.0 {
		mass := 100.0
		switch {
		case temp <= 124:
			mass = 100.0
		case temp <= 127:
			mass = 100.0 - 5.0*(temp-124)/3.0
		default:
			mass = 95.0
		}
		pts = append(pts, model.Point{Temp: temp, Value: mass})
	}
	return pts
}
