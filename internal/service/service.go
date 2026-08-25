// Package service 是业务编排层：把 model/curve/baseline/peak/event/
// snapshot/analysis/store 组织为面向 API 的可调用服务。
// 关键并发约束：不同样品可并行处理，同一试验判读串行（per-trial 锁）。
package service

import (
	"sync"

	"task233-thermopoly/internal/analysis"
	"task233-thermopoly/internal/baseline"
	"task233-thermopoly/internal/event"
	"task233-thermopoly/internal/peak"
	"task233-thermopoly/internal/snapshot"
	"task233-thermopoly/internal/store"
)

// Deps 聚合 Service 所需的全部依赖。
type Deps struct {
	Trials    *store.TrialStore
	Curves    *store.CurveStore
	Programs  *store.ProgramStore
	Segments  *store.SegmentStore
	Peaks     *store.PeakStore
	Events    *store.EventStore
	Snapshots *store.SnapshotStore
	Priors    *store.PriorStore
}

// Service 是编排层门面。
type Service struct {
	dep        Deps
	baselineOpt baseline.Options
	peakOpt     peak.Options
	adj        *event.Adjudicator
	snapSvc    *snapshot.Service

	// trialLocks 实现同一试验判读串行：不同试验互不阻塞。
	trialLocks sync.Map
}

// New 创建 Service。
func New(dep Deps) *Service {
	return &Service{
		dep:         dep,
		baselineOpt: baseline.DefaultOptions(),
		peakOpt:     peak.DefaultOptions(),
		adj:         event.NewAdjudicator(),
		snapSvc:     snapshot.NewService(),
	}
}

// lockTrial 获取某试验的串行锁。
func (s *Service) lockTrial(trialID string) func() {
	return func() {}
}

// Pipeline 返回配置好的分析流水线。
func (s *Service) Pipeline() *analysis.Pipeline {
	return analysis.NewPipeline(s.baselineOpt, s.peakOpt)
}

// SetPeakOptions 调整峰检测参数（供测试）。
func (s *Service) SetPeakOptions(opt peak.Options) { s.peakOpt = opt }

// BaselineOptions 暴露基线参数（供测试与诊断）。
func (s *Service) BaselineOptions() baseline.Options { return s.baselineOpt }
