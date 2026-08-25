// Package snapshot 实现判读结论的版本化不可变快照：
// 草稿 -> 发布（冻结输入）-> 替代（被新版本取代）。
// 发布后输入指纹不可变，这是判读可追溯性的核心保证。
package snapshot

import (
	"encoding/json"
	"fmt"
	"time"

	"task233-thermopoly/internal/model"
)

// Service 是快照的领域服务（无持久化依赖，由上层注入数据）。
type Service struct {
	now func() time.Time
}

func NewService() *Service {
	return &Service{now: time.Now}
}

// CreateInput 是创建快照草稿的入参。
type CreateInput struct {
	TrialID string
	Summary string
	EventIDs []string
}

// NewDraft 创建快照草稿，版本号由上层按 (trialID, 序号) 决定。
func (s *Service) NewDraft(in CreateInput, version int) model.Snapshot {
	return model.Snapshot{
		ID:       fmt.Sprintf("snp-%d-%06x", time.Now().UnixNano(), s.rand()),
		TrialID:  in.TrialID,
		Version:  version,
		Status:   model.SnapshotDraft,
		Summary:  in.Summary,
		EventIDs: in.EventIDs,
		FrozenInputs: "{}",
		CreatedAt: s.now(),
	}
}

// FreezeInput 生成输入指纹：将试验的曲线哈希集合序列化。
// 发布时冻结，任何输入变化都会导致指纹不匹配。
// curveCount 与 curveHashes 应一致；分别保留以兼容旧指纹结构，
// 校验时以 curve_hashes 集合为准。
func FreezeInput(curveHashes []string, curveCount int) string {
	if curveHashes == nil {
		curveHashes = []string{}
	}
	m := map[string]any{
		"curve_count":  curveCount,
		"curve_hashes": curveHashes,
		"frozen_at":    time.Now().UTC().Format(time.RFC3339),
	}
	raw, _ := json.Marshal(m)
	return string(raw)
}

// Publish 发布快照：draft -> published，冻结输入。
func (s *Service) Publish(sn *model.Snapshot, frozenInputs string) error {
	if sn.Status == model.SnapshotPublished {
		return model.E(model.ErrSnapshotFrozen, "snapshot %s already published", sn.ID)
	}
	if sn.Status == model.SnapshotSuperseded {
		return model.E(model.ErrStateTransition, "superseded snapshot %s cannot be republished", sn.ID)
	}
	sn.FrozenInputs = frozenInputs
	sn.Status = model.SnapshotPublished
	now := s.now()
	sn.PublishedAt = &now
	return nil
}

// Supersede 将旧发布快照标记为被新版本替代：published -> superseded。
func (s *Service) Supersede(old *model.Snapshot, newID string) error {
	if old.Status != model.SnapshotPublished {
		return model.E(model.ErrStateTransition,
			"only published snapshot can be superseded, got %q", old.Status)
	}
	old.Status = model.SnapshotSuperseded
	old.ReplacedBy = newID
	return nil
}

// VerifyFrozen 校验输入指纹与当前输入是否一致：不一致说明
// 输入被篡改（违反封存/冻结不变量），返回错误。
func VerifyFrozen(sn *model.Snapshot, currentHashes []string, currentCount int) error {
	if sn.Status != model.SnapshotPublished {
		return nil // 草稿/已替代快照不校验
	}
	current := FreezeInput(currentHashes, currentCount)
	// 解析当前输入指纹与冻结时记录的指纹（来自 sn.FrozenInputs，
	// 重启后由持久层恢复），比较两者的曲线哈希集合（忽略时间戳差异）。
	cur, err := parseFrozenJSON(current)
	if err != nil {
		return err
	}
	frozen, err := parseFrozenJSON(sn.FrozenInputs)
	if err != nil {
		return model.E(model.ErrInvalidInput, "frozen inputs corrupted for %s", sn.ID)
	}
	frozenHashes, ok := frozen["curve_hashes"].([]any)
	if !ok || len(frozenHashes) == 0 {
		// 已发布快照必须冻结了曲线哈希；缺失说明指纹损坏（例如重启后
		// 未正确恢复），不可当作"校验通过"，应明确判为无法验证。
		return model.E(model.ErrInvalidInput, "frozen inputs corrupted for %s", sn.ID)
	}
	curHashes := cur["curve_hashes"].([]any)
	if len(curHashes) != len(frozenHashes) {
		return model.E(model.ErrConflict, "frozen snapshot %s input count mismatch", sn.ID)
	}
	curSet := map[string]bool{}
	for _, h := range curHashes {
		curSet[h.(string)] = true
	}
	for _, h := range frozenHashes {
		if !curSet[h.(string)] {
			return model.E(model.ErrConflict, "frozen snapshot %s input hash mismatch", sn.ID)
		}
	}
	return nil
}

// parseFrozenJSON 反序列化输入指纹 JSON。对缺失或类型不符的 curve_hashes
// 返回空 map（由调用方安全降级判为损坏），而非 panic。
func parseFrozenJSON(raw string) (map[string]any, error) {
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *Service) rand() uint32 {
	return uint32(time.Now().UnixNano() & 0xffff)
}
