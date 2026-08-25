package store

import (
	"testing"

	"task233-thermopoly/internal/model"
	"task233-thermopoly/internal/snapshot"
)

// TestSnapshotPreservesFrozenInputsAfterReload 防止回归：发布的快照冻结指纹
// （含曲线哈希）必须在持久层往返后完整保留，使重启后能对未变更输入完成校验。
// 回归症状：scanSnapshot 用占位 "{}" 覆盖 FrozenInputs，重启后校验把未变更
// 快照当成损坏/无法验证。
func TestSnapshotPreservesFrozenInputsAfterReload(t *testing.T) {
	st, err := Open("")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer st.Close()
	snapStore := NewSnapshotStore(st.DB())

	hashes := []string{"hash-a", "hash-b"}
	frozen := snapshot.FreezeInput(hashes, len(hashes))
	// 确认冻结指纹确实含哈希（防护 FreezeInput 回归）。
	if frozen == "{}" {
		t.Fatalf("FreezeInput produced empty fingerprint")
	}

	sn := model.Snapshot{
		ID:           "snp-reload",
		TrialID:      "trl-1",
		Version:      1,
		Status:       model.SnapshotPublished,
		Summary:      "published",
		EventIDs:     []string{"e1", "e2"},
		FrozenInputs: frozen,
	}
	if err := snapStore.Insert(&sn); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// 更新发布状态（模拟 PublishSnapshot 持久化路径）。
	sn.Summary = "published v2"
	sn.FrozenInputs = frozen
	if err := snapStore.Update(&sn); err != nil {
		t.Fatalf("update: %v", err)
	}

	// 单条读取（Get，走 scanSnapshot）必须保留冻结指纹。
	got, err := snapStore.Get(sn.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.FrozenInputs == "{}" {
		t.Fatalf("Get clobbered FrozenInputs to {} (reload would break verification)")
	}
	if got.FrozenInputs != frozen {
		t.Fatalf("FrozenInputs round-trip mismatch:\n got=%s\nwant=%s", got.FrozenInputs, frozen)
	}

	// 列表读取（ListByTrial，走 scanSnapshotRow）同样必须保留。
	listed, err := snapStore.ListByTrial("trl-1")
	if err != nil || len(listed) != 1 {
		t.Fatalf("list: %v len=%d", err, len(listed))
	}
	if listed[0].FrozenInputs != frozen {
		t.Fatalf("ListByTrial FrozenInputs mismatch: got=%s", listed[0].FrozenInputs)
	}

	// 模拟重启：用同一冻结指纹对未变更输入校验，必须通过。
	reloaded := &model.Snapshot{
		ID:           got.ID,
		Status:       model.SnapshotPublished,
		FrozenInputs: got.FrozenInputs,
	}
	if err := snapshot.VerifyFrozen(reloaded, hashes, len(hashes)); err != nil {
		t.Fatalf("verify unchanged after reload: %v", err)
	}
}
