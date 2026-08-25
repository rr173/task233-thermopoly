package event

import (
	"time"

	"task233-thermopoly/internal/model"
)

// Adjudicator 负责事件裁决：确认/否决/标重叠，以及重叠事件拆分。
// 裁决是状态机流转的守门人，非法流转一律拒绝。
type Adjudicator struct {
	now func() time.Time
}

func NewAdjudicator() *Adjudicator {
	return &Adjudicator{now: time.Now}
}

// Adjudicate 执行一次裁决：
// candidate -> confirmed | vetoed | overlapping。
func (a *Adjudicator) Adjudicate(e *model.Event, target string, note string) error {
	if e.Status != model.EventCandidate && e.Status != model.EventOverlapping {
		return model.E(model.ErrStateTransition,
			"cannot adjudicate event in status %q to %q", e.Status, target)
	}
	switch target {
	case model.EventConfirmed:
		if e.Status == model.EventOverlapping {
			return model.E(model.ErrOverlapUnresolved,
				"overlapping event %s must be split with evidence before confirmation", e.ID)
		}
	case model.EventVetoed, model.EventOverlapping:
		// 允许
	default:
		return model.E(model.ErrInvalidInput, "unsupported adjudication target %q", target)
	}
	e.Status = target
	e.Note = note
	e.UpdatedAt = a.now()
	return nil
}

// SplitInput 是重叠事件拆分入参：将两个重叠峰对应的候选事件
// 用补充证据拆分为独立事件。
type SplitInput struct {
	EventA   model.Event
	EventB   model.Event
	Evidence string // 补充证据说明（如 TGA 微分曲线、XRD 佐证）
	FormA    string
	FormB    string
}

// Split 拆分重叠事件：两个重叠候选各自转为独立候选，
// 若拆分证据不充分（空证据）则拒绝，保证"重叠峰必须返回不确定性"。
func (a *Adjudicator) Split(in SplitInput) (model.Event, model.Event, error) {
	if in.EventA.Status != model.EventOverlapping && in.EventB.Status != model.EventOverlapping {
		return model.Event{}, model.Event{}, model.E(model.ErrStateTransition,
			"split requires both events overlapping")
	}
	if len(in.Evidence) < 4 {
		return model.Event{}, model.Event{}, model.E(model.ErrInvalidInput,
			"split requires concrete evidence, got %d chars", len(in.Evidence))
	}
	now := a.now()
	ea := in.EventA
	ea.Status = model.EventCandidate
	ea.Form = in.FormA
	ea.Note = "split from overlap: " + in.Evidence
	ea.UpdatedAt = now
	eb := in.EventB
	eb.Status = model.EventCandidate
	eb.Form = in.FormB
	eb.Note = "split from overlap: " + in.Evidence
	eb.UpdatedAt = now
	return ea, eb, nil
}

// SetOverlap 把重叠峰对应的候选事件统一标记为 overlapping（不确定），
// 使试验进入 needs_review 状态等待人工复核。
func (a *Adjudicator) SetOverlap(events []*model.Event, peaks []model.Peak) {
	overlapPeak := map[string]bool{}
	for _, p := range peaks {
		if p.Overlap {
			overlapPeak[p.ID] = true
		}
	}
	now := a.now()
	for _, e := range events {
		if overlapPeak[e.PeakID] && e.Status == model.EventCandidate {
			e.Status = model.EventOverlapping
			e.Note = "overlapping peak uncertainty requires review"
			e.UpdatedAt = now
		}
	}
}
