package snapshot

import (
	"testing"

	"task233-thermopoly/internal/model"
)

func TestBug09_MalformedFrozenInputsReturnErrorInsteadOfPanic(t *testing.T) {
	sn := &model.Snapshot{ID: "snapshot-corrupt", Status: model.SnapshotPublished, FrozenInputs: `{}`}
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("malformed frozen inputs panicked: %v", recovered)
		}
	}()
	if err := VerifyFrozen(sn, []string{"curve-hash"}, 1); err == nil {
		t.Fatal("malformed frozen inputs should return a validation error")
	}
}
