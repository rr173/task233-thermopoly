package service

import (
	"time"

	"task233-thermopoly/internal/baseline"
	"task233-thermopoly/internal/curve"
	"task233-thermopoly/internal/event"
	"task233-thermopoly/internal/model"
	"task233-thermopoly/internal/peak"
)

// BaselineResult 是基线校正的对外结果（段 + 曲线状态）。
type BaselineResult struct {
	Segments []model.Segment `json:"segments"`
	Curves   []model.Curve   `json:"curves"`
}

// RunBaseline 对某试验的 DSC/TGA 曲线执行基线校正：
// 生成校正段，异常曲线标记 anomalous，正常曲线标记 baseline_corrected。
// 同一试验串行执行。
func (s *Service) RunBaseline(trialID string) (*BaselineResult, error) {
	unlock := s.lockTrial(trialID)
	defer unlock()
	t, err := s.dep.Trials.Get(trialID)
	if err != nil {
		return nil, err
	}
	if t.Status == model.TrialSealed {
		return nil, model.E(model.ErrSealedTrial, "sealed trial %s: baseline frozen", trialID)
	}
	curves, err := s.dep.Curves.ListByTrial(trialID)
	if err != nil {
		return nil, err
	}
	if len(curves) == 0 {
		return nil, model.E(model.ErrInvalidInput, "trial %s has no curves", trialID)
	}
	res := &BaselineResult{}
	for _, c := range curves {
		cr, err := baseline.Correct(&c, s.baselineOpt)
		if err != nil {
			return nil, err
		}
		// 质量评估
		anomalous, reason := baseline.DetectAnomaly(cr.Corrected, signalScale(c))
		status := model.SegmentBaselineCorrected
		if anomalous {
			status = model.SegmentAnomalous
		}
		seg := model.Segment{
			ID:        newID("seg"),
			TrialID:   trialID,
			CurveID:   c.ID,
			Kind:      c.Kind,
			Status:    status,
			Baseline:  baseline.ParamsJSON(map[string]any{"method": "linear", "anomaly": reason}),
			Params:    baseline.ParamsJSON(cr.Params),
			CreatedAt: time.Now(),
		}
		if err := s.dep.Segments.Insert(&seg); err != nil {
			return nil, err
		}
		_ = s.dep.Curves.UpdateStatus(c.ID, status)
		c.Status = status
		res.Curves = append(res.Curves, c)
		res.Segments = append(res.Segments, seg)
	}
	return res, nil
}

// DetectPeaks 对某试验的 DSC 曲线执行峰检测并持久化峰区间。
func (s *Service) DetectPeaks(trialID string) ([]model.Peak, error) {
	unlock := s.lockTrial(trialID)
	defer unlock()
	t, err := s.dep.Trials.Get(trialID)
	if err != nil {
		return nil, err
	}
	if t.Status == model.TrialSealed {
		return nil, model.E(model.ErrSealedTrial, "sealed trial %s: peaks frozen", trialID)
	}
	dscCurves, err := s.dep.Curves.ListByKind(trialID, model.CurveDSC)
	if err != nil {
		return nil, err
	}
	if len(dscCurves) == 0 {
		return nil, model.E(model.ErrInvalidInput, "trial %s has no DSC curves", trialID)
	}
	var peaks []model.Peak
	for _, c := range dscCurves {
		det, err := peak.Detect(&c, s.peakOpt)
		if err != nil {
			return nil, err
		}
		for _, p := range det.Peaks {
			p.ID = newID("pk")
			p.TrialID = trialID
			p.CurveID = c.ID
			p.Status = "detected"
			p.CreatedAt = time.Now()
			peaks = append(peaks, p)
		}
	}
	if err := s.dep.Peaks.InsertMany(peaks); err != nil {
		return nil, err
	}
	// 峰检测完成后试验进入 pending_review（若还在 receiving）
	if t.Status == model.TrialReceiving {
		_ = s.dep.Trials.UpdateStatus(trialID, model.TrialPending, t.CurveHash)
	}
	return peaks, nil
}

// ListPeaks 列出某试验的全部峰。
func (s *Service) ListPeaks(trialID string) ([]model.Peak, error) {
	return s.dep.Peaks.ListByTrial(trialID)
}

// GenerateEvents 对已检测峰应用晶型先验生成事件候选；
// 重叠峰对应事件标记为 overlapping，试验进入 needs_review。
func (s *Service) GenerateEvents(trialID string) ([]model.Event, error) {
	unlock := s.lockTrial(trialID)
	defer unlock()
	t, err := s.dep.Trials.Get(trialID)
	if err != nil {
		return nil, err
	}
	if t.Status == model.TrialSealed {
		return nil, model.E(model.ErrSealedTrial, "sealed trial %s: events frozen", trialID)
	}
	priors, err := s.dep.Priors.ActiveList()
	if err != nil {
		return nil, err
	}
	tgaCurves, err := s.dep.Curves.ListByKind(trialID, model.CurveTGA)
	if err != nil {
		return nil, err
	}
	peaks, err := s.dep.Peaks.ListByTrial(trialID)
	if err != nil {
		return nil, err
	}
	if len(peaks) == 0 {
		return nil, model.E(model.ErrInvalidInput, "trial %s has no detected peaks", trialID)
	}
	cl := event.NewClassifier(priors)
	var tga *model.Curve
	if len(tgaCurves) > 0 {
		tga = &tgaCurves[0]
	}
	refTemp := 0.0
	if len(peaks) > 0 {
		// 参考温度取首峰起点（近似曲线起点）
		refTemp = peaks[0].StartTemp
	}
	hasOverlap := false
	var events []model.Event
	for _, pk := range peaks {
		massLoss := 0.0
		hasTGA := tga != nil
		if hasTGA {
			massLoss = model.AlignMassLoss(tga.Points, pk.PeakTemp, refTemp)
		}
		cand, err := cl.Classify(event.ClassifyInput{
			Peak:        pk,
			MassLossPct: massLoss,
			HasTGA:      hasTGA,
		})
		if err != nil {
			return nil, err
		}
		ev := cand.Event
		ev.ID = newID("evt")
		ev.TrialID = trialID
		ev.Evidence = event.EvidenceJSON(cand.Evidence)
		if pk.Overlap && ev.Status == model.EventCandidate {
			ev.Status = model.EventOverlapping
			hasOverlap = true
		}
		now := time.Now()
		ev.CreatedAt = now
		ev.UpdatedAt = now
		events = append(events, ev)
	}
	if err := s.dep.Events.InsertMany(events); err != nil {
		return nil, err
	}
	// 状态推进：有重叠 -> needs_review；否则 -> pending_review（若仍在接收）
	target := model.TrialPending
	if hasOverlap {
		target = model.TrialNeedsReview
	}
	if model.CanTransition(t.Status, target) && t.Status != target {
		_ = s.dep.Trials.UpdateStatus(trialID, target, t.CurveHash)
	}
	return events, nil
}

// AdjudicateEvent 裁决单个事件：candidate -> confirmed | vetoed | overlapping。
func (s *Service) AdjudicateEvent(eventID, target, note string) (*model.Event, error) {
	e, err := s.dep.Events.Get(eventID)
	if err != nil {
		return nil, err
	}
	unlock := s.lockTrial(e.TrialID)
	defer unlock()
	t, err := s.dep.Trials.Get(e.TrialID)
	if err != nil {
		return nil, err
	}
	if t.Status == model.TrialSealed {
		return nil, model.E(model.ErrSealedTrial, "sealed trial %s: events frozen", e.TrialID)
	}
	if err := s.adj.Adjudicate(e, target, note); err != nil {
		return nil, err
	}
	if err := s.dep.Events.Update(e); err != nil {
		return nil, err
	}
	// 裁决后自动推进状态：全部确认 -> confirmed；存在未决/否决 -> needs_review
	if err := s.refreshTrialStatus(e.TrialID); err != nil {
		return nil, err
	}
	return e, nil
}

// SplitOverlapping 拆分重叠事件（补充证据后拆为独立候选）。
func (s *Service) SplitOverlapping(eventAID, eventBID, evidence, formA, formB string) ([]model.Event, error) {
	ea, err := s.dep.Events.Get(eventAID)
	if err != nil {
		return nil, err
	}
	eb, err := s.dep.Events.Get(eventBID)
	if err != nil {
		return nil, err
	}
	if ea.TrialID != eb.TrialID {
		return nil, model.E(model.ErrInvalidInput, "events belong to different trials")
	}
	unlock := s.lockTrial(ea.TrialID)
	defer unlock()
	t, err := s.dep.Trials.Get(ea.TrialID)
	if err != nil {
		return nil, err
	}
	if t.Status == model.TrialSealed {
		return nil, model.E(model.ErrSealedTrial, "sealed trial %s: events frozen", ea.TrialID)
	}
	na, nb, err := s.adj.Split(event.SplitInput{
		EventA:   *ea,
		EventB:   *eb,
		Evidence: evidence,
		FormA:    formA,
		FormB:    formB,
	})
	if err != nil {
		return nil, err
	}
	if err := s.dep.Events.Update(&na); err != nil {
		return nil, err
	}
	if err := s.dep.Events.Update(&nb); err != nil {
		return nil, err
	}
	if err := s.refreshTrialStatus(ea.TrialID); err != nil {
		return nil, err
	}
	return []model.Event{na, nb}, nil
}

// ListEvents 列出某试验的事件。
func (s *Service) ListEvents(trialID string) ([]model.Event, error) {
	return s.dep.Events.ListByTrial(trialID)
}

// refreshTrialStatus 依据事件状态自动推进试验状态：
// 存在 overlapping -> needs_review；全部 confirmed 且无重叠 -> confirmed；否则 pending。
func (s *Service) refreshTrialStatus(trialID string) error {
	t, err := s.dep.Trials.Get(trialID)
	if err != nil {
		return err
	}
	if t.Status == model.TrialSealed {
		return nil
	}
	events, err := s.dep.Events.ListByTrial(trialID)
	if err != nil {
		return err
	}
	if len(events) == 0 {
		return nil
	}
	hasOverlap := false
	hasPending := false
	for _, e := range events {
		switch e.Status {
		case model.EventOverlapping:
			hasOverlap = true
		case model.EventCandidate, model.EventUnknown:
			hasPending = true
		}
	}
	target := model.TrialPending
	switch {
	case hasOverlap || hasPending:
		target = model.TrialNeedsReview
	default:
		target = model.TrialConfirmed
	}
	if model.CanTransition(t.Status, target) && t.Status != target {
		return s.dep.Trials.UpdateStatus(trialID, target, t.CurveHash)
	}
	return nil
}

// signalScale 估计曲线信号尺度（用于异常评估）。
func signalScale(c model.Curve) float64 {
	if len(c.Points) == 0 {
		return 1
	}
	minV, maxV := c.Points[0].Value, c.Points[0].Value
	for _, p := range c.Points[1:] {
		if p.Value < minV {
			minV = p.Value
		}
		if p.Value > maxV {
			maxV = p.Value
		}
	}
	scale := maxV - minV
	if scale <= 0 {
		scale = 1
	}
	return scale
}

var _ = curve.MassChangePct // 保留引用（curve 包仍被 curve.go 使用）
