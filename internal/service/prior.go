package service

import (
	"time"

	"task233-thermopoly/internal/model"
)

// CreatePriorInput 是创建晶型先验的入参。
type CreatePriorInput struct {
	FormFrom       string  `json:"form_from"`
	FormTo         string  `json:"form_to"`
	OnsetLow       float64 `json:"onset_low"`
	OnsetHigh      float64 `json:"onset_high"`
	Direction      string  `json:"direction"`
	MaxMassLossPct float64 `json:"max_mass_loss_pct"`
	Note           string  `json:"note"`
}

// CreatePrior 创建先验知识（默认启用）。
func (s *Service) CreatePrior(in CreatePriorInput) (*model.PolymorphPrior, error) {
	if in.FormFrom == "" || in.FormTo == "" {
		return nil, model.E(model.ErrInvalidInput, "form_from and form_to are required")
	}
	if in.OnsetHigh <= in.OnsetLow {
		return nil, model.E(model.ErrInvalidInput,
			"onset window invalid: high %v must exceed low %v", in.OnsetHigh, in.OnsetLow)
	}
	if in.Direction != model.DirectionEndothermic && in.Direction != model.DirectionExothermic {
		return nil, model.E(model.ErrInvalidInput, "invalid direction %q", in.Direction)
	}
	maxLoss := in.MaxMassLossPct
	if maxLoss == 0 {
		maxLoss = 0.5 // 晶型转变默认几乎无质量损失
	}
	p := &model.PolymorphPrior{
		ID:             newID("pri"),
		FormFrom:       in.FormFrom,
		FormTo:         in.FormTo,
		OnsetLow:       in.OnsetLow,
		OnsetHigh:      in.OnsetHigh,
		Direction:      in.Direction,
		MaxMassLossPct: maxLoss,
		Note:           in.Note,
		Active:         true,
		CreatedAt:      time.Now(),
	}
	if err := s.dep.Priors.Insert(p); err != nil {
		return nil, err
	}
	return p, nil
}

// UpdatePrior 更新先验（含启用/停用）。
func (s *Service) UpdatePrior(id string, in CreatePriorInput, active *bool) (*model.PolymorphPrior, error) {
	p, err := s.dep.Priors.Get(id)
	if err != nil {
		return nil, err
	}
	if in.FormFrom != "" {
		p.FormFrom = in.FormFrom
	}
	if in.FormTo != "" {
		p.FormTo = in.FormTo
	}
	if in.OnsetHigh > in.OnsetLow {
		p.OnsetLow = in.OnsetLow
		p.OnsetHigh = in.OnsetHigh
	}
	if in.Direction != "" {
		if in.Direction != model.DirectionEndothermic && in.Direction != model.DirectionExothermic {
			return nil, model.E(model.ErrInvalidInput, "invalid direction %q", in.Direction)
		}
		p.Direction = in.Direction
	}
	if in.MaxMassLossPct > 0 {
		p.MaxMassLossPct = in.MaxMassLossPct
	}
	if in.Note != "" {
		p.Note = in.Note
	}
	if active != nil {
		p.Active = *active
	}
	if err := s.dep.Priors.Update(p); err != nil {
		return nil, err
	}
	return p, nil
}

// ListPriors 列出全部先验（含停用项）。
func (s *Service) ListPriors() ([]model.PolymorphPrior, error) {
	return s.dep.Priors.List()
}

// GetPrior 取单个先验。
func (s *Service) GetPrior(id string) (*model.PolymorphPrior, error) {
	return s.dep.Priors.Get(id)
}

// Stats 汇总系统统计（供 /api/stats）。
func (s *Service) Stats() (map[string]any, error) {
	trialCounts, err := s.dep.Trials.CountByStatus()
	if err != nil {
		return nil, err
	}
	priorCount, err := s.dep.Priors.Count()
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"trials_by_status": trialCounts,
		"priors":           priorCount,
		"generated_at":     time.Now().UTC().Format(time.RFC3339),
	}, nil
}
