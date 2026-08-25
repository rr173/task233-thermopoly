package httpapi

import (
	"net/http"

	"task233-thermopoly/internal/service"
)

// listPriors GET /api/priors —— 列出全部晶型先验。
func (h *Handler) listPriors(w http.ResponseWriter, r *http.Request) {
	priors, err := h.svc.ListPriors()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, priors)
}

// createPrior POST /api/priors —— 创建晶型先验。
func (h *Handler) createPrior(w http.ResponseWriter, r *http.Request) {
	var in service.CreatePriorInput
	if err := readJSON(w, r, &in); err != nil {
		return
	}
	p, err := h.svc.CreatePrior(in)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

// getPrior GET /api/priors/{id} —— 取单个先验。
func (h *Handler) getPrior(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.GetPrior(r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// updatePrior PATCH /api/priors/{id} —— 更新先验（可启用/停用）。
func (h *Handler) updatePrior(w http.ResponseWriter, r *http.Request) {
	var in service.CreatePriorInput
	if err := readJSON(w, r, &in); err != nil {
		return
	}
	var active *bool
	if v, ok := r.URL.Query()["active"]; ok && len(v) > 0 {
		b := v[0] == "1" || v[0] == "true"
		active = &b
	}
	p, err := h.svc.UpdatePrior(r.PathValue("id"), in, active)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// stats GET /api/stats —— 系统统计。
func (h *Handler) stats(w http.ResponseWriter, r *http.Request) {
	st, err := h.svc.Stats()
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// health GET /api/health —— 健康检查。
func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"service": "task233-thermopoly",
	})
}
