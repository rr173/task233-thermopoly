package store

import (
	"database/sql"
	"time"

	"task233-thermopoly/internal/model"
)

// PeakStore 提供峰区间的持久化操作。
type PeakStore struct{ db *sql.DB }

// NewPeakStore 创建峰仓库。
func NewPeakStore(db *sql.DB) *PeakStore { return &PeakStore{db: db} }

// InsertMany 批量写入峰（单事务，失败整体回滚）。
func (s *PeakStore) InsertMany(peaks []model.Peak) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`
		INSERT INTO peaks (id, trial_id, curve_id, start_idx, end_idx, start_temp, end_temp,
			peak_temp, peak_value, direction, height, area, separation, overlap, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for i := range peaks {
		p := &peaks[i]
		if _, err := stmt.Exec(p.ID, p.TrialID, p.CurveID, p.StartIdx, p.EndIdx,
			p.StartTemp, p.EndTemp, p.PeakTemp, p.PeakValue, p.Direction,
			p.Height, p.Area, p.Separation, boolToInt(p.Overlap), p.Status, ts(p.CreatedAt)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListByTrial 列出某试验的全部峰（按峰顶温度排序）。
func (s *PeakStore) ListByTrial(trialID string) ([]model.Peak, error) {
	rows, err := s.db.Query(`
		SELECT id, trial_id, curve_id, start_idx, end_idx, start_temp, end_temp,
			peak_temp, peak_value, direction, height, area, separation, overlap, status, created_at
		FROM peaks WHERE trial_id = ? ORDER BY peak_temp`, trialID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPeaks(rows)
}

// Get 按 ID 取峰。
func (s *PeakStore) Get(id string) (*model.Peak, error) {
	row := s.db.QueryRow(`
		SELECT id, trial_id, curve_id, start_idx, end_idx, start_temp, end_temp,
			peak_temp, peak_value, direction, height, area, separation, overlap, status, created_at
		FROM peaks WHERE id = ?`, id)
	var p model.Peak
	var created string
	err := row.Scan(&p.ID, &p.TrialID, &p.CurveID, &p.StartIdx, &p.EndIdx,
		&p.StartTemp, &p.EndTemp, &p.PeakTemp, &p.PeakValue, &p.Direction,
		&p.Height, &p.Area, &p.Separation, &p.Overlap, &p.Status, &created)
	if err != nil {
		return nil, wrapNotFound(err)
	}
	p.CreatedAt = parseTs(created)
	return &p, nil
}

// UpdateStatus 更新峰状态。
func (s *PeakStore) UpdateStatus(id, status string) error {
	_, err := s.db.Exec(`UPDATE peaks SET status = ? WHERE id = ?`, status, id)
	return err
}

// Count 统计某试验峰数。
func (s *PeakStore) Count(trialID string) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM peaks WHERE trial_id = ?`, trialID).Scan(&n)
	return n, err
}

func scanPeaks(rows interface {
	Next() bool
	Scan(...any) error
	Close() error
	Err() error
}) ([]model.Peak, error) {
	defer rows.Close()
	var out []model.Peak
	for rows.Next() {
		var p model.Peak
		var created string
		if err := rows.Scan(&p.ID, &p.TrialID, &p.CurveID, &p.StartIdx, &p.EndIdx,
			&p.StartTemp, &p.EndTemp, &p.PeakTemp, &p.PeakValue, &p.Direction,
			&p.Height, &p.Area, &p.Separation, &p.Overlap, &p.Status, &created); err != nil {
			return nil, err
		}
		// 注意：Scan 已将 INTEGER 0/1 转换到 bool（database/sql 支持），
		// 不能再调 intToBool 覆盖——否则 bool 值会被当作 int 判等导致恒 false。
		p.CreatedAt = parseTs(created)
		out = append(out, p)
	}
	return out, rows.Err()
}

var _ = time.Now