package service

import (
	"time"

	"task233-thermopoly/internal/model"
	"task233-thermopoly/internal/snapshot"
)

// CreateSnapshotInput 是创建快照草稿的入参。
type CreateSnapshotInput struct {
	TrialID string   `json:"trial_id"`
	Summary string   `json:"summary"`
	EventIDs []string `json:"event_ids"`
}

// CreateSnapshot 创建快照草稿（draft），版本号自动递增。
func (s *Service) CreateSnapshot(in CreateSnapshotInput) (*model.Snapshot, error) {
	unlock := s.lockTrial(in.TrialID)
	defer unlock()
	if _, err := s.dep.Trials.Get(in.TrialID); err != nil {
		return nil, err
	}
	// 校验事件 ID 都属于该试验
	events, err := s.dep.Events.ListByTrial(in.TrialID)
	if err != nil {
		return nil, err
	}
	valid := map[string]bool{}
	for _, e := range events {
		valid[e.ID] = true
	}
	if len(events) == 0 {
		return nil, model.E(model.ErrInvalidInput, "trial %s has no adjudicated events", in.TrialID)
	}
	seen := map[string]bool{}
	for _, id := range in.EventIDs {
		if !valid[id] {
			return nil, model.E(model.ErrInvalidInput,
				"event %s does not belong to trial %s", id, in.TrialID)
		}
		seen[id] = true
	}
	if len(seen) != len(events) {
		return nil, model.E(model.ErrInvalidInput,
			"snapshot for trial %s must include every event", in.TrialID)
	}
	version, err := s.dep.Snapshots.NextVersion(in.TrialID)
	if err != nil {
		return nil, err
	}
	sn := s.snapSvc.NewDraft(snapshot.CreateInput{
		TrialID:  in.TrialID,
		Summary:  in.Summary,
		EventIDs: in.EventIDs,
	}, version)
	if err := s.dep.Snapshots.Insert(&sn); err != nil {
		return nil, err
	}
	return &sn, nil
}

// PublishSnapshot 发布快照：draft -> published，冻结输入指纹；
// 若试验已有发布快照，旧版本标记为 superseded。
func (s *Service) PublishSnapshot(id string) (*model.Snapshot, error) {
	sn, err := s.dep.Snapshots.Get(id)
	if err != nil {
		return nil, err
	}
	unlock := s.lockTrial(sn.TrialID)
	defer unlock()
	t, err := s.dep.Trials.Get(sn.TrialID)
	if err != nil {
		return nil, err
	}
	// 快照发布要求事件已裁决（不允许带未确认/重叠事件发布）
	events, err := s.dep.Events.ListByTrial(sn.TrialID)
	if err != nil {
		return nil, err
	}
	for _, e := range events {
		if e.Status == model.EventOverlapping || e.Status == model.EventCandidate {
			return nil, model.E(model.ErrOverlapUnresolved,
				"cannot publish snapshot %s: event %s not resolved", id, e.ID)
		}
	}
	// 冻结输入：曲线哈希集合
	hashes, err := s.dep.Curves.AllHashes(sn.TrialID)
	if err != nil {
		return nil, err
	}
	frozen := snapshot.FreezeInput(hashes, len(hashes))
	// 替代旧发布版本
	if old, err := s.dep.Snapshots.Published(sn.TrialID); err == nil {
		if old.ID != sn.ID {
			if err := s.snapSvc.Supersede(old, sn.ID); err != nil {
				return nil, err
			}
			if err := s.dep.Snapshots.Update(old); err != nil {
				return nil, err
			}
		}
	}
	if err := s.snapSvc.Publish(sn, frozen); err != nil {
		return nil, err
	}
	if err := s.dep.Snapshots.Update(sn); err != nil {
		return nil, err
	}
	// 发布后试验推进到 confirmed（若仍为 pending/needs_review）
	if model.CanTransition(t.Status, model.TrialConfirmed) && t.Status != model.TrialConfirmed {
		_ = s.dep.Trials.UpdateStatus(sn.TrialID, model.TrialConfirmed, t.CurveHash)
	}
	return sn, nil
}

// GetSnapshot 取快照详情。
func (s *Service) GetSnapshot(id string) (*model.Snapshot, error) {
	return s.dep.Snapshots.Get(id)
}

// ListSnapshots 列出某试验的快照。
func (s *Service) ListSnapshots(trialID string) ([]model.Snapshot, error) {
	return s.dep.Snapshots.ListByTrial(trialID)
}

// VerifySnapshotInput 校验已发布快照的输入冻结性：
// 若当前曲线哈希与冻结指纹不一致，返回冲突错误。
func (s *Service) VerifySnapshotInput(id string) error {
	sn, err := s.dep.Snapshots.Get(id)
	if err != nil {
		return err
	}
	if sn.Status != model.SnapshotPublished {
		return model.E(model.ErrStateTransition,
			"snapshot %s is not published; only published snapshots verify inputs", id)
	}
	hashes, err := s.dep.Curves.AllHashes(sn.TrialID)
	if err != nil {
		return err
	}
	return snapshot.VerifyFrozen(sn, hashes, len(hashes))
}

var _ = time.Now
