package httpapi

import (
	"net/http"

	"task233-thermopoly/internal/model"
	"task233-thermopoly/internal/service"
)

// runBaseline POST /api/trials/{id}/baseline —— 执行基线校正。
func (h *Handler) runBaseline(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.RunBaseline(r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// detectPeaks POST /api/trials/{id}/peaks/detect —— 峰检测。
func (h *Handler) detectPeaks(w http.ResponseWriter, r *http.Request) {
	peaks, err := h.svc.DetectPeaks(r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, peaks)
}

// listPeaks GET /api/trials/{id}/peaks —— 列出峰区间。
func (h *Handler) listPeaks(w http.ResponseWriter, r *http.Request) {
	peaks, err := h.svc.ListPeaks(r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, peaks)
}

// generateEvents POST /api/trials/{id}/events/generate —— 晶型先验判读生成事件候选。
func (h *Handler) generateEvents(w http.ResponseWriter, r *http.Request) {
	events, err := h.svc.GenerateEvents(r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, events)
}

// listEvents GET /api/trials/{id}/events —— 列出事件。
func (h *Handler) listEvents(w http.ResponseWriter, r *http.Request) {
	events, err := h.svc.ListEvents(r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, events)
}

// adjudicateEvent PATCH /api/events/{id} —— 事件裁决。
// body: {"target": "confirmed|vetoed|overlapping", "note": "..."}
func (h *Handler) adjudicateEvent(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Target string `json:"target"`
		Note   string `json:"note"`
	}
	if err := readJSON(w, r, &body); err != nil {
		return
	}
	e, err := h.svc.AdjudicateEvent(r.PathValue("id"), body.Target, body.Note)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, e)
}

// splitEvent POST /api/events/{id}/split —— 拆分重叠事件（补充证据）。
// body: {"pair_id": "...", "evidence": "...", "form_a": "...", "form_b": "..."}
func (h *Handler) splitEvent(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PairID   string `json:"pair_id"`
		Evidence string `json:"evidence"`
		FormA    string `json:"form_a"`
		FormB    string `json:"form_b"`
	}
	if err := readJSON(w, r, &body); err != nil {
		return
	}
	if body.PairID == "" {
		writeErr(w, model.E(model.ErrInvalidInput, "pair_id (the overlapping counterpart event) is required"))
		return
	}
	events, err := h.svc.SplitOverlapping(r.PathValue("id"), body.PairID, body.Evidence, body.FormA, body.FormB)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, events)
}

// createSnapshot POST /api/trials/{id}/snapshots —— 创建快照草稿。
func (h *Handler) createSnapshot(w http.ResponseWriter, r *http.Request) {
	var in service.CreateSnapshotInput
	if err := readJSON(w, r, &in); err != nil {
		return
	}
	in.TrialID = r.PathValue("id")
	sn, err := h.svc.CreateSnapshot(in)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, sn)
}

// listSnapshots GET /api/trials/{id}/snapshots —— 列出快照。
func (h *Handler) listSnapshots(w http.ResponseWriter, r *http.Request) {
	sns, err := h.svc.ListSnapshots(r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sns)
}

// getSnapshot GET /api/snapshots/{id} —— 快照详情。
func (h *Handler) getSnapshot(w http.ResponseWriter, r *http.Request) {
	sn, err := h.svc.GetSnapshot(r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sn)
}

// publishSnapshot POST /api/snapshots/{id}/publish —— 发布快照（冻结输入）。
func (h *Handler) publishSnapshot(w http.ResponseWriter, r *http.Request) {
	sn, err := h.svc.PublishSnapshot(r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sn)
}

// verifySnapshot GET /api/snapshots/{id}/verify —— 校验快照输入冻结性。
func (h *Handler) verifySnapshot(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.VerifySnapshotInput(r.PathValue("id")); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"verified": true})
}
