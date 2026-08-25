package httpapi

import (
	"net/http"
	"strconv"

	"task233-thermopoly/internal/model"
	"task233-thermopoly/internal/service"
)

// createTrial POST /api/trials —— 创建试验（接收中）。
func (h *Handler) createTrial(w http.ResponseWriter, r *http.Request) {
	var in service.CreateTrialInput
	if err := readJSON(w, r, &in); err != nil {
		return
	}
	t, err := h.svc.CreateTrial(in)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

// listTrials GET /api/trials?status=&limit= —— 列出试验。
func (h *Handler) listTrials(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	ts, err := h.svc.ListTrials(status, limit)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ts)
}

// getTrial GET /api/trials/{id} —— 试验详情。
func (h *Handler) getTrial(w http.ResponseWriter, r *http.Request) {
	t, err := h.svc.GetTrial(r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

// transitionTrial PATCH /api/trials/{id} —— 推进状态机。
// body: {"status": "confirmed"}
func (h *Handler) transitionTrial(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Status string `json:"status"`
	}
	if err := readJSON(w, r, &body); err != nil {
		return
	}
	t, err := h.svc.TransitionTrial(r.PathValue("id"), body.Status)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

// sealTrial POST /api/trials/{id}/seal —— 封存试验（终态）。
func (h *Handler) sealTrial(w http.ResponseWriter, r *http.Request) {
	t, err := h.svc.SealTrial(r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

// setProgram PUT /api/trials/{id}/program —— 设置升温程序。
func (h *Handler) setProgram(w http.ResponseWriter, r *http.Request) {
	var in service.SetProgramInput
	if err := readJSON(w, r, &in); err != nil {
		return
	}
	in.TrialID = r.PathValue("id")
	p, err := h.svc.SetProgram(in)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

// getProgram GET /api/trials/{id}/program —— 取激活升温程序。
func (h *Handler) getProgram(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.GetProgram(r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// importCurve POST /api/trials/{id}/curves —— 导入 DSC/TGA 曲线（哈希幂等）。
func (h *Handler) importCurve(w http.ResponseWriter, r *http.Request) {
	var in service.ImportCurveInput
	if err := readJSON(w, r, &in); err != nil {
		return
	}
	in.TrialID = r.PathValue("id")
	if in.Unit == "" {
		in.Unit = model.UnitCelsius
	}
	c, err := h.svc.ImportCurve(in)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

// listCurves GET /api/trials/{id}/curves —— 列出试验曲线。
func (h *Handler) listCurves(w http.ResponseWriter, r *http.Request) {
	curves, err := h.svc.ListCurves(r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, curves)
}

// getCurve GET /api/curves/{id} —— 取单条曲线。
func (h *Handler) getCurve(w http.ResponseWriter, r *http.Request) {
	c, err := h.svc.GetCurve(r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// listSegments GET /api/trials/{id}/segments —— 列出分析段。
func (h *Handler) listSegments(w http.ResponseWriter, r *http.Request) {
	segs, err := h.svc.ListSegments(r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, segs)
}
