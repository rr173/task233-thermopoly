package service

import (
	"testing"

	"task233-thermopoly/internal/model"
)

// TestSetProgramVersionLifecycle 验证升温程序版本生命周期：
// 连续更新应生成递增的新版本、停用旧版本，并始终返回最新的唯一激活版本。
func TestSetProgramVersionLifecycle(t *testing.T) {
	svc := newTestService(t)
	tr, err := svc.CreateTrial(CreateTrialInput{Name: "p", Material: "FormA", Unit: model.UnitCelsius})
	if err != nil {
		t.Fatalf("create trial: %v", err)
	}

	prog := func(rate float64) SetProgramInput {
		return SetProgramInput{
			TrialID: tr.ID, Name: "std", StartTemp: 30, EndTemp: 200, RateKPerMin: rate,
		}
	}

	// 首次设置 -> version 1
	p1, err := svc.SetProgram(prog(10))
	if err != nil {
		t.Fatalf("set program v1: %v", err)
	}
	if p1.Version != 1 || !p1.IsActive {
		t.Fatalf("first program = %+v, want version=1 active", p1)
	}

	// 连续更新应生成递增的新版本
	p2, err := svc.SetProgram(prog(20))
	if err != nil {
		t.Fatalf("set program v2: %v", err)
	}
	if p2.Version != 2 {
		t.Fatalf("second program version = %d, want 2", p2.Version)
	}
	p3, err := svc.SetProgram(prog(30))
	if err != nil {
		t.Fatalf("set program v3: %v", err)
	}
	if p3.Version != 3 {
		t.Fatalf("third program version = %d, want 3", p3.Version)
	}

	// GetProgram 始终返回最新激活版本（v3, rate 30）
	got, err := svc.GetProgram(tr.ID)
	if err != nil {
		t.Fatalf("get program: %v", err)
	}
	if got.Version != 3 || got.RateKPerMin != 30 || !got.IsActive {
		t.Fatalf("active program = %+v, want version=3 rate=30 active", got)
	}

	// 旧版本必须在数据库中停用：v3 为唯一激活版本
	if p1.ID == p2.ID || p2.ID == p3.ID {
		t.Fatalf("each update must insert a new program row: %s %s %s", p1.ID, p2.ID, p3.ID)
	}
	for _, old := range []string{p1.ID, p2.ID} {
		prev, err := svc.dep.Programs.Get(old)
		if err != nil {
			t.Fatalf("reload %s: %v", old, err)
		}
		if prev.IsActive {
			t.Fatalf("old program %s (v%d) must be inactive in db", old, prev.Version)
		}
	}
	// 重复版本不可存在：直接写一条 version=2 应被唯一约束拒绝
	if err := svc.dep.Programs.Upsert(&model.Program{
		ID: "prg_dup", TrialID: tr.ID, Name: "dup", StartTemp: 1, EndTemp: 2,
		RateKPerMin: 1, Version: 2, IsActive: true,
	}); err == nil {
		t.Fatal("duplicate (trial_id, version) must be rejected by unique constraint")
	}

	// 再取一次激活程序，确认仍为 v3（重复 GetProgram 不漂移）
	got2, err := svc.GetProgram(tr.ID)
	if err != nil {
		t.Fatalf("get program again: %v", err)
	}
	if got2.Version != got.Version {
		t.Fatalf("active program drifted: %d -> %d", got.Version, got2.Version)
	}
}
