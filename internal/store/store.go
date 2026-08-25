// Package store 提供 SQLite 持久化层：建表迁移、CRUD 与查询。
// 所有时间以 RFC3339 存储；点列/证据等结构以 JSON 序列化入 TEXT 列。
// 并发写由 SQLite 事务与上层 per-trial 锁共同保证。
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"

	"task233-thermopoly/internal/model"
)

// Store 是持久化门面：持有连接与各子仓库。
type Store struct {
	db *sql.DB
}

// Open 打开（或创建）SQLite 数据库并执行建表迁移。
// path 为空时使用内存库（供测试与 smoke-test）。
func Open(path string) (*Store, error) {
	dsn := ":memory:"
	if path != "" {
		dsn = fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", path)
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite 单写者：串行化写事务
	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &Store{db: db}, nil
}

// Close 关闭连接。
func (s *Store) Close() error { return s.db.Close() }

// DB 暴露底层连接（供事务编排使用）。
func (s *Store) DB() *sql.DB { return s.db }

func migrate(db *sql.DB) error {
	schema := []string{
		`CREATE TABLE IF NOT EXISTS trials (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			material TEXT NOT NULL,
			batch_no TEXT,
			description TEXT,
			status TEXT NOT NULL,
			unit TEXT NOT NULL DEFAULT 'C',
			curve_hash TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			sealed_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS programs (
			id TEXT PRIMARY KEY,
			trial_id TEXT NOT NULL,
			name TEXT,
			start_temp REAL NOT NULL,
			end_temp REAL NOT NULL,
			rate_k_per_min REAL NOT NULL,
			version INTEGER NOT NULL DEFAULT 1,
			is_active INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			FOREIGN KEY(trial_id) REFERENCES trials(id)
		)`,
		`CREATE TABLE IF NOT EXISTS curves (
			id TEXT PRIMARY KEY,
			trial_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			name TEXT,
			unit TEXT NOT NULL,
			sample_interval REAL NOT NULL,
			points TEXT NOT NULL,
			hash TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'raw',
			imported_at TEXT NOT NULL,
			mass_change_pct REAL,
			FOREIGN KEY(trial_id) REFERENCES trials(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_curves_hash ON curves(trial_id, hash)`,
		`CREATE TABLE IF NOT EXISTS segments (
			id TEXT PRIMARY KEY,
			trial_id TEXT NOT NULL,
			curve_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			status TEXT NOT NULL,
			baseline TEXT,
			params TEXT,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS peaks (
			id TEXT PRIMARY KEY,
			trial_id TEXT NOT NULL,
			curve_id TEXT NOT NULL,
			start_idx INTEGER NOT NULL,
			end_idx INTEGER NOT NULL,
			start_temp REAL NOT NULL,
			end_temp REAL NOT NULL,
			peak_temp REAL NOT NULL,
			peak_value REAL NOT NULL,
			direction TEXT NOT NULL,
			height REAL NOT NULL,
			area REAL NOT NULL,
			separation REAL NOT NULL DEFAULT 1,
			overlap INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'detected',
			created_at TEXT NOT NULL,
			FOREIGN KEY(trial_id) REFERENCES trials(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_peaks_trial ON peaks(trial_id)`,
		`CREATE TABLE IF NOT EXISTS events (
			id TEXT PRIMARY KEY,
			trial_id TEXT NOT NULL,
			peak_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			form TEXT NOT NULL,
			onset_temp REAL NOT NULL,
			peak_temp REAL NOT NULL,
			confidence REAL NOT NULL,
			status TEXT NOT NULL,
			evidence TEXT NOT NULL,
			note TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(trial_id) REFERENCES trials(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_events_trial ON events(trial_id)`,
		`CREATE TABLE IF NOT EXISTS snapshots (
			id TEXT PRIMARY KEY,
			trial_id TEXT NOT NULL,
			version INTEGER NOT NULL,
			status TEXT NOT NULL,
			summary TEXT,
			event_ids TEXT NOT NULL,
			frozen_inputs TEXT NOT NULL,
			published_at TEXT,
			replaced_by TEXT,
			created_at TEXT NOT NULL,
			UNIQUE(trial_id, version)
		)`,
		`CREATE TABLE IF NOT EXISTS priors (
			id TEXT PRIMARY KEY,
			form_from TEXT NOT NULL,
			form_to TEXT NOT NULL,
			onset_low REAL NOT NULL,
			onset_high REAL NOT NULL,
			direction TEXT NOT NULL,
			max_mass_loss_pct REAL NOT NULL DEFAULT 0.5,
			note TEXT,
			active INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL
		)`,
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("exec schema: %w (%s)", err, stmt[:60])
		}
	}
	return nil
}

// 时间序列化帮助

func ts(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func parseTs(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func tsPtr(t *time.Time) any {
	if t == nil {
		return nil
	}
	return ts(*t)
}

func parseTsPtr(v any) *time.Time {
	if v == nil {
		return nil
	}
	if s, ok := v.(string); ok {
		t := parseTs(s)
		return &t
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func intToBool(v any) bool { return asInt(v) == 1 }

func asInt(v any) int {
	switch n := v.(type) {
	case int64:
		return int(n)
	case int:
		return n
	}
	return 0
}

func jsonString(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}

func jsonUnmarshal(s string, v any) error {
	return json.Unmarshal([]byte(s), v)
}

// isNotFound 判断错误是否为无行。
func isNotFound(err error) bool { return err == sql.ErrNoRows }

// wrapNotFound 把 ErrNoRows 转为领域 ErrNotFound。
func wrapNotFound(err error) error {
	if isNotFound(err) {
		return model.ErrNotFound
	}
	return err
}
