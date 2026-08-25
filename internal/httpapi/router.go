// Package httpapi 提供 HTTP API 层：路由注册、JSON 编解码与错误映射。
// 所有路由以 /api 前缀，业务能力见各 *_api.go。
package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"task233-thermopoly/internal/model"
	"task233-thermopoly/internal/service"
)

// Handler 聚合 service 依赖并实现 http.Handler。
type Handler struct {
	svc *service.Service
	mux *http.ServeMux
}

// New 创建 HTTP 处理器并注册全部路由。
func New(svc *service.Service) *Handler {
	h := &Handler{svc: svc, mux: http.NewServeMux()}
	h.registerRoutes()
	return h
}

// ServeHTTP 实现 http.Handler。
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) registerRoutes() {
	// 试验
	h.mux.HandleFunc("POST /api/trials", h.createTrial)
	h.mux.HandleFunc("GET /api/trials", h.listTrials)
	h.mux.HandleFunc("GET /api/trials/{id}", h.getTrial)
	h.mux.HandleFunc("PATCH /api/trials/{id}", h.transitionTrial)
	h.mux.HandleFunc("POST /api/trials/{id}/seal", h.sealTrial)
	// 升温程序
	h.mux.HandleFunc("PUT /api/trials/{id}/program", h.setProgram)
	h.mux.HandleFunc("GET /api/trials/{id}/program", h.getProgram)
	// 曲线与段
	h.mux.HandleFunc("POST /api/trials/{id}/curves", h.importCurve)
	h.mux.HandleFunc("GET /api/trials/{id}/curves", h.listCurves)
	h.mux.HandleFunc("GET /api/curves/{id}", h.getCurve)
	h.mux.HandleFunc("GET /api/trials/{id}/segments", h.listSegments)
	// 分析
	h.mux.HandleFunc("POST /api/trials/{id}/baseline", h.runBaseline)
	h.mux.HandleFunc("POST /api/trials/{id}/peaks/detect", h.detectPeaks)
	h.mux.HandleFunc("GET /api/trials/{id}/peaks", h.listPeaks)
	h.mux.HandleFunc("POST /api/trials/{id}/events/generate", h.generateEvents)
	h.mux.HandleFunc("GET /api/trials/{id}/events", h.listEvents)
	h.mux.HandleFunc("PATCH /api/events/{id}", h.adjudicateEvent)
	h.mux.HandleFunc("POST /api/events/{id}/split", h.splitEvent)
	// 快照
	h.mux.HandleFunc("POST /api/trials/{id}/snapshots", h.createSnapshot)
	h.mux.HandleFunc("GET /api/trials/{id}/snapshots", h.listSnapshots)
	h.mux.HandleFunc("GET /api/snapshots/{id}", h.getSnapshot)
	h.mux.HandleFunc("POST /api/snapshots/{id}/publish", h.publishSnapshot)
	h.mux.HandleFunc("GET /api/snapshots/{id}/verify", h.verifySnapshot)
	// 先验与统计
	h.mux.HandleFunc("GET /api/priors", h.listPriors)
	h.mux.HandleFunc("POST /api/priors", h.createPrior)
	h.mux.HandleFunc("GET /api/priors/{id}", h.getPrior)
	h.mux.HandleFunc("PATCH /api/priors/{id}", h.updatePrior)
	h.mux.HandleFunc("GET /api/stats", h.stats)
	h.mux.HandleFunc("GET /api/health", h.health)
}

// writeJSON 输出 JSON 响应。
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("httpapi: encode response: %v", err)
	}
}

// writeErr 把领域错误映射为 HTTP 状态码与消息。
func writeErr(w http.ResponseWriter, err error) {
	var de *model.DomainError
	status := http.StatusInternalServerError
	if errors.As(err, &de) {
		switch de.Kind {
		case model.ErrNotFound:
			status = http.StatusNotFound
		case model.ErrConflict, model.ErrCurveDuplicate, model.ErrSnapshotFrozen:
			status = http.StatusConflict
		case model.ErrSealedTrial, model.ErrStateTransition, model.ErrTrialNotReady:
			status = http.StatusConflict
		case model.ErrOverlapUnresolved:
			status = http.StatusUnprocessableEntity
		default:
			status = http.StatusBadRequest
		}
	} else if errors.Is(err, model.ErrNotFound) {
		status = http.StatusNotFound
	}
	writeJSON(w, status, map[string]any{"error": err.Error()})
}

// readJSON 解析请求体 JSON 到目标结构。
func readJSON(w http.ResponseWriter, r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeErr(w, model.E(model.ErrInvalidInput, "invalid JSON body: %v", err))
		return err
	}
	return nil
}
