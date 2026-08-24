// Package event 实现晶型转变事件判读：将检测到的峰与晶型先验知识匹配，
// 区分晶型转变、溶剂脱除与熔融，并支持人工裁决（确认/否决）与重叠拆分。
package event

import (
	"encoding/json"

	"task233-thermopoly/internal/model"
)

// Classifier 是事件分类器：持有先验知识库，对峰给出事件候选。
type Classifier struct {
	priors []model.PolymorphPrior
}

// NewClassifier 创建分类器；priors 为启用的晶型先验列表。
func NewClassifier(priors []model.PolymorphPrior) *Classifier {
	return &Classifier{priors: priors}
}

// ClassifyInput 是分类输入：一个峰 + 对齐后的 TGA 证据。
type ClassifyInput struct {
	Peak         model.Peak
	MassLossPct  float64 // 峰顶温度处相对质量损失（%）
	HasTGA       bool    // 是否有 TGA 曲线可佐证
}

// Candidate 是分类输出：事件候选 + 证据。
type Candidate struct {
	Event    model.Event
	Evidence model.Evidence
}

// Classify 对单个峰执行晶型先验判定：
// 1. 遍历启用先验，匹配温度窗口 + 热流方向 + 质量损失上限；
// 2. 最优匹配形成 polymorph 事件；3. 无匹配时按证据归类。
func (cl *Classifier) Classify(in ClassifyInput) (Candidate, error) {
	if in.Peak.Direction != model.DirectionEndothermic && in.Peak.Direction != model.DirectionExothermic {
		return Candidate{}, model.E(model.ErrInvalidInput, "invalid peak direction %q", in.Peak.Direction)
	}
	ev := model.Evidence{
		PeakTemp:    in.Peak.PeakTemp,
		Direction:   in.Peak.Direction,
		MassLossPct: in.MassLossPct,
	}
	best := -1
	bestScore := 0.0
	for i, p := range cl.priors {
		if !p.Active {
			continue
		}
		inWindow := in.Peak.PeakTemp >= p.OnsetLow && in.Peak.PeakTemp <= p.OnsetHigh
		dirMatch := p.Direction == in.Peak.Direction
		massOK := in.MassLossPct <= p.MaxMassLossPct
		// 晶型转变的先验本质是无质量损失：质量损失超限即硬性排除，
		// 即便温度窗口与热流方向匹配（否则 126°C 失重峰会被误判为晶型转变）。
		if !massOK {
			continue
		}
		score := 0.0
		if inWindow {
			score += 0.5
		}
		if dirMatch {
			score += 0.3
		}
		if massOK {
			score += 0.2
		}
		if score > bestScore {
			bestScore = score
			best = i
		}
	}
	if best >= 0 && bestScore >= 0.8 {
		p := cl.priors[best]
		ev.MatchedPrior = p.ID
		ev.InOnsetWindow = in.Peak.PeakTemp >= p.OnsetLow && in.Peak.PeakTemp <= p.OnsetHigh
		ev.DirectionMatch = p.Direction == in.Peak.Direction
		ev.MassLossOK = in.MassLossPct <= p.MaxMassLossPct
		ev.Reasons = append(ev.Reasons,
			"peak matches prior "+p.FormFrom+"->"+p.FormTo)
		e := model.Event{
			PeakID:     in.Peak.ID,
			Kind:       model.EventPolymorph,
			Form:       p.FormFrom + "->" + p.FormTo,
			OnsetTemp:  p.OnsetLow,
			PeakTemp:   in.Peak.PeakTemp,
			Confidence: bestScore,
			Status:     model.EventCandidate,
		}
		return Candidate{Event: e, Evidence: ev}, nil
	}
	return cl.classifyByThermalEvidence(in, ev)
}

// classifyByThermalEvidence 无先验匹配时，按热证据归因：
// 显著质量损失 -> 溶剂脱除；吸热且无质量损失 -> 熔融；否则未知。
func (cl *Classifier) classifyByThermalEvidence(in ClassifyInput, ev model.Evidence) (Candidate, error) {
	switch {
	case in.HasTGA && in.MassLossPct > 1.0:
		ev.Reasons = append(ev.Reasons,
			"mass loss "+jsonNum(in.MassLossPct)+"% exceeds 1% threshold")
		e := model.Event{
			PeakID:     in.Peak.ID,
			Kind:       model.EventDesolvation,
			Form:       "solvent/water removal",
			OnsetTemp:  in.Peak.StartTemp,
			PeakTemp:   in.Peak.PeakTemp,
			Confidence: 0.6,
			Status:     model.EventCandidate,
		}
		return Candidate{Event: e, Evidence: ev}, nil
	case in.Peak.Direction == model.DirectionEndothermic:
		ev.Reasons = append(ev.Reasons, "endothermic peak without mass loss")
		e := model.Event{
			PeakID:     in.Peak.ID,
			Kind:       model.EventFusion,
			Form:       "melting",
			OnsetTemp:  in.Peak.StartTemp,
			PeakTemp:   in.Peak.PeakTemp,
			Confidence: 0.5,
			Status:     model.EventCandidate,
		}
		return Candidate{Event: e, Evidence: ev}, nil
	default:
		ev.Reasons = append(ev.Reasons, "no prior match and ambiguous thermal evidence")
		e := model.Event{
			PeakID:     in.Peak.ID,
			Kind:       model.EventUnknown,
			Form:       "unassigned",
			OnsetTemp:  in.Peak.StartTemp,
			PeakTemp:   in.Peak.PeakTemp,
			Confidence: 0.3,
			Status:     model.EventCandidate,
		}
		return Candidate{Event: e, Evidence: ev}, nil
	}
}

// EvidenceJSON 序列化证据摘要。
func EvidenceJSON(ev model.Evidence) string {
	raw, err := json.Marshal(ev)
	if err != nil {
		return `{"error":"serialize failed"}`
	}
	return string(raw)
}

func jsonNum(v float64) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}
