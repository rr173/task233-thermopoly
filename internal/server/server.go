// Package server 组装 HTTP 服务：加载持久化、构造 service、注册路由。
package server

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"task233-thermopoly/internal/httpapi"
	"task233-thermopoly/internal/service"
	"task233-thermopoly/internal/store"
)

// Config 是服务启动配置。
type Config struct {
	Addr string // 监听地址（如 :8080）
	DB   string // SQLite 文件路径（空=内存）
}

// Server 是组装后的 HTTP 服务。
type Server struct {
	cfg    Config
	store  *store.Store
	svc    *service.Service
	http   *http.Server
	api    *httpapi.Handler
}

// New 创建 Server（打开数据库并初始化依赖，但不启动监听）。
func New(cfg Config) (*Server, error) {
	st, err := store.Open(cfg.DB)
	if err != nil {
		return nil, err
	}
	dep := service.Deps{
		Trials:    store.NewTrialStore(st.DB()),
		Curves:    store.NewCurveStore(st.DB()),
		Programs:  store.NewProgramStore(st.DB()),
		Segments:  store.NewSegmentStore(st.DB()),
		Peaks:     store.NewPeakStore(st.DB()),
		Events:    store.NewEventStore(st.DB()),
		Snapshots: store.NewSnapshotStore(st.DB()),
		Priors:    store.NewPriorStore(st.DB()),
	}
	svc := service.New(dep)
	api := httpapi.New(svc)
	s := &Server{
		cfg:   cfg,
		store: st,
		svc:   svc,
		api:   api,
	}
	s.http = &http.Server{
		Addr:              cfg.Addr,
		Handler:           api,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return s, nil
}

// Store 暴露持久化门面（供 smoke-test 直接访问）。
func (s *Server) Store() *store.Store { return s.store }

// Service 暴露 service 层（供 smoke-test 直接访问）。
func (s *Server) Service() *service.Service { return s.svc }

// Handler 暴露 HTTP 处理器（供测试直接调用）。
func (s *Server) Handler() http.Handler { return s.api }

// ListenAndServe 启动长驻 HTTP 服务，阻塞直至退出。
func (s *Server) ListenAndServe() error {
	log.Printf("task233-thermopoly listening on %s (db=%s)", s.cfg.Addr, s.cfg.DB)
	return s.http.ListenAndServe()
}

// Shutdown 优雅关闭。
func (s *Server) Shutdown(ctx context.Context) error {
	if err := s.http.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return s.store.Close()
}
