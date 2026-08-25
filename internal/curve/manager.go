package curve

import "task233-thermopoly/internal/model"

// Manager 协调曲线级操作：导入、幂等判重、状态推进与质量统计。
// 它依赖 Importer 完成校验，对外暴露领域级方法供 service 层调用。
type Manager struct {
	importer *Importer
	dupCheck func(trialID, hash string) (bool, error) // 注入的幂等查询钩子
}

// NewManager 创建曲线管理器；dupCheck 由持久化层注入。
func NewManager(dupCheck func(trialID, hash string) (bool, error)) *Manager {
	return &Manager{importer: NewImporter(), dupCheck: dupCheck}
}

// ImportOrDedupe 导入曲线；若同试验已有相同内容哈希，返回 ErrCurveDuplicate。
// 这是"同一曲线哈希幂等"不变量在入口处的第一道闸。
func (m *Manager) ImportOrDedupe(trialUnit string, in ImportInput) (*model.Curve, error) {
	c, err := m.importer.Import(trialUnit, in)
	if err != nil {
		return nil, err
	}
	if m.dupCheck != nil {
		dup, err := m.dupCheck(in.TrialID, c.Hash)
		if err != nil {
			return nil, err
		}
		if dup {
			return nil, model.E(model.ErrCurveDuplicate, "curve with hash %s already imported for trial %s", c.Hash, in.TrialID)
		}
	}
	return c, nil
}

// IsDuplicate 便捷判重接口（供测试与 service 复用）。
func (m *Manager) IsDuplicate(trialID, hash string) (bool, error) {
	if m.dupCheck == nil {
		return false, nil
	}
	return m.dupCheck(trialID, hash)
}

// MarkStatus 推进曲线状态（raw -> baseline_corrected / anomalous / duplicate）。
func MarkStatus(c *model.Curve, status string) error {
	allowed := map[string]bool{
		model.SegmentRaw:              true,
		model.SegmentBaselineCorrected: true,
		model.SegmentAnomalous:        true,
		model.SegmentDuplicate:        true,
	}
	if !allowed[status] {
		return model.E(model.ErrInvalidInput, "illegal curve status %q", status)
	}
	c.Status = status
	return nil
}

// Fingerprint 返回曲线的稳定指纹（哈希 + 点数 + 温度范围），
// 供快照冻结输入比对：任何输入变化都会改变指纹。
func Fingerprint(c *model.Curve) map[string]any {
	return map[string]any{
		"hash":        c.Hash,
		"points":      len(c.Points),
		"kind":        c.Kind,
		"start_temp":  c.Points[0].Temp,
		"end_temp":    c.Points[len(c.Points)-1].Temp,
	}
}
