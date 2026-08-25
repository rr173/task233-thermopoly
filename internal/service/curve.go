package service

import (
	"time"

	"task233-thermopoly/internal/curve"
	"task233-thermopoly/internal/model"
)

// ImportCurveInput 是导入曲线入参。
type ImportCurveInput struct {
	TrialID string       `json:"trial_id"`
	Kind    string       `json:"kind"`
	Name    string       `json:"name"`
	Unit    string       `json:"unit"`
	Points  []model.Point `json:"points"`
}

// ImportCurve 导入曲线：哈希幂等判重、单位一致性校验、封存试验拒绝修改。
// 导入成功后试验状态若为 receiving 保持不变；曲线是输入，导入不推进状态。
func (s *Service) ImportCurve(in ImportCurveInput) (*model.Curve, error) {
	unlock := s.lockTrial(in.TrialID)
	defer unlock()
	t, err := s.dep.Trials.Get(in.TrialID)
	if err != nil {
		return nil, err
	}
	mgr := curve.NewManager(s.dep.Curves.HashExists)
	c, err := mgr.ImportOrDedupe(t.Unit, curve.ImportInput{
		TrialID: in.TrialID,
		Kind:    in.Kind,
		Name:    in.Name,
		Unit:    in.Unit,
		Points:  in.Points,
	})
	if err != nil {
		return nil, err
	}
	if c.Kind == model.CurveTGA {
		mc := curve.MassChangePct(c.Points)
		c.MassChangePct = &mc
	}
	c.ImportedAt = time.Now()
	if err := s.dep.Curves.Insert(c); err != nil {
		return nil, err
	}
	// 更新试验曲线指纹（输入摘要）
	hashes, err := s.dep.Curves.AllHashes(in.TrialID)
	if err == nil {
		_ = s.dep.Trials.SetCurveHash(in.TrialID, joinHashes(hashes))
	}
	return c, nil
}

// ListCurves 列出某试验全部曲线。
func (s *Service) ListCurves(trialID string) ([]model.Curve, error) {
	if _, err := s.dep.Trials.Get(trialID); err != nil {
		return nil, err
	}
	return s.dep.Curves.ListByTrial(trialID)
}

// GetCurve 取单条曲线。
func (s *Service) GetCurve(id string) (*model.Curve, error) {
	return s.dep.Curves.Get(id)
}

// SetProgramInput 是设置升温程序的入参。
type SetProgramInput struct {
	TrialID     string  `json:"trial_id"`
	Name        string  `json:"name"`
	StartTemp   float64 `json:"start_temp"`
	EndTemp     float64 `json:"end_temp"`
	RateKPerMin float64 `json:"rate_k_per_min"`
}

// SetProgram 设置（或更新）升温程序：旧版本置为不激活，新版本号递增。
func (s *Service) SetProgram(in SetProgramInput) (*model.Program, error) {
	unlock := s.lockTrial(in.TrialID)
	defer unlock()
	_, err := s.dep.Trials.Get(in.TrialID)
	if err != nil {
		return nil, err
	}
	if in.RateKPerMin <= 0 || in.EndTemp <= in.StartTemp {
		return nil, model.E(model.ErrInvalidInput,
			"invalid program: rate %v must be positive and end %v > start %v",
			in.RateKPerMin, in.EndTemp, in.StartTemp)
	}
	version := 1
	if cur, err := s.dep.Programs.GetActive(in.TrialID); err == nil {
		version = cur.Version + 1
	}
	p := &model.Program{
		ID:          newID("prg"),
		TrialID:     in.TrialID,
		Name:        in.Name,
		StartTemp:   in.StartTemp,
		EndTemp:     in.EndTemp,
		RateKPerMin: in.RateKPerMin,
		Version:     version,
		IsActive:    true,
		CreatedAt:   time.Now(),
	}
	if err := s.dep.Programs.Upsert(p); err != nil {
		return nil, err
	}
	return p, nil
}

// GetProgram 取某试验当前激活的升温程序。
func (s *Service) GetProgram(trialID string) (*model.Program, error) {
	return s.dep.Programs.GetActive(trialID)
}

// ListSegments 列出某试验的分析段。
func (s *Service) ListSegments(trialID string) ([]model.Segment, error) {
	return s.dep.Segments.ListByTrial(trialID)
}

// JoinHashes 汇总曲线哈希列表为单个指纹（供试验汇总字段）。
func joinHashes(hashes []string) string {
	out := ""
	for i, h := range hashes {
		if i > 0 {
			out += ","
		}
		out += h[:12] // 取前 12 位即可稳定标识
	}
	return out
}
