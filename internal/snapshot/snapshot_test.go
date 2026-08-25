package snapshot

import (
	"encoding/json"
	"testing"

	"task233-thermopoly/internal/model"
)

// TestFreezeInputCapturesHashes 防止回归：冻结指纹必须实际包含曲线哈希
// 集合，而不是占位空数组。否则重启后无法对未变更输入完成校验。
func TestFreezeInputCapturesHashes(t *testing.T) {
	hashes := []string{"aaa", "bbb"}
	raw := FreezeInput(hashes, len(hashes))

	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("unmarshal frozen: %v", err)
	}
	got, ok := m["curve_hashes"].([]any)
	if !ok || len(got) != 2 {
		t.Fatalf("curve_hashes not preserved: %v", m["curve_hashes"])
	}
	for i, want := range hashes {
		if got[i] != want {
			t.Errorf("hash[%d] = %v, want %s", i, got[i], want)
		}
	}
}

// TestVerifyFrozenRoundTrip 防止回归：模拟重启后从持久层恢复的快照，
// 其 FrozenInputs 必须能对未变更输入校验通过、对变更输入判不一致。
func TestVerifyFrozenRoundTrip(t *testing.T) {
	hashes := []string{"h1", "h2"}
	// 发布时冻结的指纹（持久化后重启恢复的快照携带该值）。
	frozen := FreezeInput(hashes, len(hashes))

	recovered := &model.Snapshot{
		ID:           "snp-test",
		Status:       model.SnapshotPublished,
		FrozenInputs: frozen, // 重启后由持久层恢复
	}

	// 未变更输入：校验通过。
	if err := VerifyFrozen(recovered, hashes, len(hashes)); err != nil {
		t.Fatalf("verify unchanged after restart: %v", err)
	}

	// 输入被篡改（多一条曲线）：判为冲突。
	if err := VerifyFrozen(recovered, append(hashes, "h3"), 3); err == nil {
		t.Fatal("verify tampered (extra input) should fail")
	}

	// 输入被篡改（哈希改变）：判为冲突。
	if err := VerifyFrozen(recovered, []string{"h1", "changed"}, 2); err == nil {
		t.Fatal("verify tampered (hash changed) should fail")
	}

	// 指纹损坏（空哈希集合）：明确判为损坏，而非 panic 或通过。
	corrupted := &model.Snapshot{
		ID:           "snp-corrupt",
		Status:       model.SnapshotPublished,
		FrozenInputs: FreezeInput(nil, 0), // curve_hashes 为空
	}
	if err := VerifyFrozen(corrupted, hashes, len(hashes)); err == nil {
		t.Fatal("verify corrupted (empty fingerprint) should fail, not pass")
	}
}
