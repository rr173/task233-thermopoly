package service

import (
	"time"

	"task233-thermopoly/internal/model"
)

// CreateTrialInput 是创建试验的入参。
type CreateTrialInput struct {
	Name        string `json:"name"`
	Material    string `json:"material"`
	BatchNo     string `json:"batch_no"`
	Description string `json:"description"`
	Unit        string `json:"unit"` // 温度单位（C/K）
}

// CreateTrial 创建试验（状态 receiving）。单位缺省 C。
func (s *Service) CreateTrial(in CreateTrialInput) (*model.Trial, error) {
	if in.Name == "" || in.Material == "" {
		return nil, model.E(model.ErrInvalidInput, "name and material are required")
	}
	unit := in.Unit
	if unit == "" {
		unit = model.UnitCelsius
	}
	if !model.ValidateTemperatureUnit(unit) {
		return nil, model.E(model.ErrInvalidInput, "unsupported temperature unit %q", unit)
	}
	now := time.Now()
	t := &model.Trial{
		ID:          newID("trl"),
		Name:        in.Name,
		Material:    in.Material,
		BatchNo:     in.BatchNo,
		Description: in.Description,
		Status:      model.TrialReceiving,
		Unit:        unit,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.dep.Trials.Insert(t); err != nil {
		return nil, err
	}
	return t, nil
}

// GetTrial 取试验详情。
func (s *Service) GetTrial(id string) (*model.Trial, error) {
	return s.dep.Trials.Get(id)
}

// ListTrials 按状态列出试验。
func (s *Service) ListTrials(status string, limit int) ([]model.Trial, error) {
	if status != "" && !model.ValidTrialStatus(status) {
		return nil, model.E(model.ErrInvalidInput, "invalid trial status %q", status)
	}
	return s.dep.Trials.List(status, limit)
}

// TransitionTrial 推进试验状态机：receiving -> pending_review -> needs_review -> confirmed -> sealed。
// 只允许前进，不允许回退；sealed 为终态。
func (s *Service) TransitionTrial(id, target string) (*model.Trial, error) {
	unlock := s.lockTrial(id)
	defer unlock()
	t, err := s.dep.Trials.Get(id)
	if err != nil {
		return nil, err
	}
	if !model.ValidTrialStatus(target) {
		return nil, model.E(model.ErrInvalidInput, "invalid target status %q", target)
	}
	if !model.CanTransition(t.Status, target) {
		return nil, model.E(model.ErrStateTransition,
			"cannot transition trial %s from %s to %s", id, t.Status, target)
	}
	if target == model.TrialSealed {
		if err := s.dep.Trials.Seal(id); err != nil {
			return nil, err
		}
		return s.dep.Trials.Get(id)
	}
	hash := t.CurveHash
	if err := s.dep.Trials.UpdateStatus(id, target, hash); err != nil {
		return nil, err
	}
	return s.dep.Trials.Get(id)
}

// SealTrial 封存试验：输入冻结，禁止任何修改。终态不可回退。
func (s *Service) SealTrial(id string) (*model.Trial, error) {
	unlock := s.lockTrial(id)
	defer unlock()
	t, err := s.dep.Trials.Get(id)
	if err != nil {
		return nil, err
	}
	if t.Status == model.TrialSealed {
		return t, nil
	}
	// 只有到达 confirmed 才允许封存（必须先定稿判读）
	if t.Status != model.TrialConfirmed {
		return nil, model.E(model.ErrStateTransition,
			"trial %s must be confirmed before sealing (current %s)", id, t.Status)
	}
	if err := s.dep.Trials.UpdateStatus(id, model.TrialConfirmed, t.CurveHash); err != nil {
		return nil, err
	}
	return s.dep.Trials.Get(id)
}
