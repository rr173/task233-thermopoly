package snapshot

import (
	"strings"
	"testing"

	"task233-thermopoly/internal/model"
)

func TestFreezeInputAndVerifyFrozenUnchanged(t *testing.T) {
	hashes := []string{"h-dsc", "h-tga"}
	sn := model.Snapshot{ID: "snp-1", Status: model.SnapshotDraft, FrozenInputs: "{}"}
	sn.FrozenInputs = FreezeInput(hashes, len(hashes))
	sn.Status = model.SnapshotPublished

	// 输入未变化：校验通过。
	if err := VerifyFrozen(&sn, hashes, len(hashes)); err != nil {
		t.Fatalf("unchanged input must verify: %v", err)
	}
}

// TestVerifyFrozenDetectsNewCurve 回归：快照发布后试验又导入了新曲线，
// 校验必须报告输入已变化，不能继续把结论视为有效。
func TestVerifyFrozenDetectsNewCurve(t *testing.T) {
	frozenHashes := []string{"h-dsc", "h-tga"}
	sn := model.Snapshot{ID: "snp-1", Status: model.SnapshotPublished}
	sn.FrozenInputs = FreezeInput(frozenHashes, len(frozenHashes))

	// 发布后又导入一条新曲线（哈希集合扩大）。
	current := append([]string{}, frozenHashes...)
	current = append(current, "h-new-dsc-2")
	err := VerifyFrozen(&sn, current, len(current))
	if err == nil {
		t.Fatal("verify must report changed input when a new curve is imported after publish")
	}
	if !model.IsKind(err, model.ErrConflict) {
		t.Errorf("error kind = %v, want ErrConflict", err)
	}
}

func TestVerifyFrozenDetectsRemovedCurve(t *testing.T) {
	frozenHashes := []string{"h-dsc", "h-tga"}
	sn := model.Snapshot{ID: "snp-1", Status: model.SnapshotPublished}
	sn.FrozenInputs = FreezeInput(frozenHashes, len(frozenHashes))

	// 移除一条曲线：数量减少。
	current := []string{"h-dsc"}
	if err := VerifyFrozen(&sn, current, len(current)); err == nil {
		t.Fatal("verify must report changed input when a curve is removed")
	}
}

func TestVerifyFrozenDetectsReplacedCurve(t *testing.T) {
	frozenHashes := []string{"h-dsc", "h-tga"}
	sn := model.Snapshot{ID: "snp-1", Status: model.SnapshotPublished}
	sn.FrozenInputs = FreezeInput(frozenHashes, len(frozenHashes))

	// 替换其中一条曲线：数量不变但哈希不同。
	current := []string{"h-dsc", "h-tga-replaced"}
	if err := VerifyFrozen(&sn, current, len(current)); err == nil {
		t.Fatal("verify must report changed input when a curve is replaced")
	}
}

// TestVerifyFrozenSkipsNonPublished 非发布态快照不校验。
func TestVerifyFrozenSkipsNonPublished(t *testing.T) {
	sn := model.Snapshot{ID: "snp-1", Status: model.SnapshotDraft, FrozenInputs: "{}"}
	if err := VerifyFrozen(&sn, []string{"anything"}, 1); err != nil {
		t.Fatalf("draft snapshot must not verify, got: %v", err)
	}
}

func TestVerifyFrozenCorrupted(t *testing.T) {
	sn := model.Snapshot{ID: "snp-1", Status: model.SnapshotPublished, FrozenInputs: "{not json"}
	err := VerifyFrozen(&sn, []string{"h"}, 1)
	if err == nil || !strings.Contains(err.Error(), "corrupted") {
		t.Fatalf("corrupted frozen inputs must be reported, got: %v", err)
	}
}
