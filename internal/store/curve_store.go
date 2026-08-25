package store

import (
	"database/sql"
	"time"

	"task233-thermopoly/internal/model"
)

// CurveStore 提供曲线、分析段与升温程序的持久化操作。
type CurveStore struct{ db *sql.DB }

// NewCurveStore 创建曲线仓库。
func NewCurveStore(db *sql.DB) *CurveStore { return &CurveStore{db: db} }

// Insert 写入曲线（points 序列化为 JSON）。
func (s *CurveStore) Insert(c *model.Curve) error {
	_, err := s.db.Exec(`
		INSERT INTO curves (id, trial_id, kind, name, unit, sample_interval, points, hash, status, imported_at, mass_change_pct)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.TrialID, c.Kind, c.Name, c.Unit, c.SampleInterval,
		jsonString(c.Points), c.Hash, c.Status, ts(c.ImportedAt), c.MassChangePct)
	return err
}

// Get 按 ID 取曲线。
func (s *CurveStore) Get(id string) (*model.Curve, error) {
	row := s.db.QueryRow(`
		SELECT id, trial_id, kind, name, unit, sample_interval, points, hash, status, imported_at, mass_change_pct
		FROM curves WHERE id = ?`, id)
	var c model.Curve
	var (
		points    string
		imported  string
		massLoss  sql.NullFloat64
	)
	err := row.Scan(&c.ID, &c.TrialID, &c.Kind, &c.Name, &c.Unit, &c.SampleInterval,
		&points, &c.Hash, &c.Status, &imported, &massLoss)
	if err != nil {
		return nil, wrapNotFound(err)
	}
	if err := jsonUnmarshal(points, &c.Points); err != nil {
		return nil, err
	}
	c.ImportedAt = parseTs(imported)
	if massLoss.Valid {
		c.MassChangePct = &massLoss.Float64
	}
	return &c, nil
}

// ListByTrial 列出某试验的全部曲线（按类型、导入时间排序）。
func (s *CurveStore) ListByTrial(trialID string) ([]model.Curve, error) {
	rows, err := s.db.Query(`
		SELECT id, trial_id, kind, name, unit, sample_interval, points, hash, status, imported_at, mass_change_pct
		FROM curves WHERE trial_id = ? ORDER BY kind, imported_at`, trialID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Curve
	for rows.Next() {
		var c model.Curve
		var (
			points   string
			imported string
			massLoss sql.NullFloat64
		)
		if err := rows.Scan(&c.ID, &c.TrialID, &c.Kind, &c.Name, &c.Unit, &c.SampleInterval,
			&points, &c.Hash, &c.Status, &imported, &massLoss); err != nil {
			return nil, err
		}
		if err := jsonUnmarshal(points, &c.Points); err != nil {
			return nil, err
		}
		c.ImportedAt = parseTs(imported)
		if massLoss.Valid {
			c.MassChangePct = &massLoss.Float64
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListByKind 列出某试验指定类型的曲线。
func (s *CurveStore) ListByKind(trialID, kind string) ([]model.Curve, error) {
	all, err := s.ListByTrial(trialID)
	if err != nil {
		return nil, err
	}
	out := all[:0]
	for _, c := range all {
		if c.Kind == kind {
			out = append(out, c)
		}
	}
	return out, nil
}

// HashExists 判断同试验是否存在相同内容哈希的曲线（幂等判重）。
func (s *CurveStore) HashExists(trialID, hash string) (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM curves WHERE trial_id = ? AND hash = ?`, trialID, hash).Scan(&n)
	return n > 0, err
}

// UpdateStatus 更新曲线状态（baseline_corrected / anomalous / duplicate）。
func (s *CurveStore) UpdateStatus(id, status string) error {
	_, err := s.db.Exec(`UPDATE curves SET status = ? WHERE id = ?`, status, id)
	return err
}

// AllHashes 返回某试验全部曲线的哈希列表（供快照冻结输入）。
func (s *CurveStore) AllHashes(trialID string) ([]string, error) {
	rows, err := s.db.Query(`SELECT hash FROM curves WHERE trial_id = ? ORDER BY kind, imported_at LIMIT 1`, trialID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// ProgramStore 升温程序仓库。

type ProgramStore struct{ db *sql.DB }

// NewProgramStore 创建升温程序仓库。
func NewProgramStore(db *sql.DB) *ProgramStore { return &ProgramStore{db: db} }

// Upsert 写入（或覆盖激活）升温程序：旧版本置为不激活。
func (s *ProgramStore) Upsert(p *model.Program) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE programs SET is_active = 0 WHERE trial_id = ? AND is_active = 1`, p.TrialID); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		INSERT INTO programs (id, trial_id, name, start_temp, end_temp, rate_k_per_min, version, is_active, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.TrialID, p.Name, p.StartTemp, p.EndTemp, p.RateKPerMin, p.Version, boolToInt(p.IsActive), ts(p.CreatedAt)); err != nil {
		return err
	}
	return tx.Commit()
}

// GetActive 取试验当前激活的程序。
func (s *ProgramStore) GetActive(trialID string) (*model.Program, error) {
	row := s.db.QueryRow(`
		SELECT id, trial_id, name, start_temp, end_temp, rate_k_per_min, version, is_active, created_at
		FROM programs WHERE trial_id = ? AND is_active = 1 ORDER BY version DESC LIMIT 1`, trialID)
	var p model.Program
	var created string
	err := row.Scan(&p.ID, &p.TrialID, &p.Name, &p.StartTemp, &p.EndTemp,
		&p.RateKPerMin, &p.Version, &p.IsActive, &created)
	if err != nil {
		return nil, wrapNotFound(err)
	}
	// Scan 已把 INTEGER 转 bool，勿再 intToBool 覆盖
	p.CreatedAt = parseTs(created)
	return &p, nil
}

// SegmentStore 分析段仓库。

type SegmentStore struct{ db *sql.DB }

// NewSegmentStore 创建分析段仓库。
func NewSegmentStore(db *sql.DB) *SegmentStore { return &SegmentStore{db: db} }

// Insert 写入分析段。
func (s *SegmentStore) Insert(seg *model.Segment) error {
	_, err := s.db.Exec(`
		INSERT INTO segments (id, trial_id, curve_id, kind, status, baseline, params, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		seg.ID, seg.TrialID, seg.CurveID, seg.Kind, seg.Status, seg.Baseline, seg.Params, ts(seg.CreatedAt))
	return err
}

// ListByTrial 列出某试验的分析段。
func (s *SegmentStore) ListByTrial(trialID string) ([]model.Segment, error) {
	rows, err := s.db.Query(`
		SELECT id, trial_id, curve_id, kind, status, baseline, params, created_at
		FROM segments WHERE trial_id = ? ORDER BY created_at`, trialID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Segment
	for rows.Next() {
		var seg model.Segment
		var created string
		if err := rows.Scan(&seg.ID, &seg.TrialID, &seg.CurveID, &seg.Kind, &seg.Status,
			&seg.Baseline, &seg.Params, &created); err != nil {
			return nil, err
		}
		seg.CreatedAt = parseTs(created)
		out = append(out, seg)
	}
	return out, rows.Err()
}

// IncompleteDetections 查询待恢复的未完成峰检测（重启恢复语义）：
// 返回已有曲线但无任何峰记录的试验 ID 列表。
func (s *CurveStore) IncompleteDetections(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`
		SELECT DISTINCT c.trial_id FROM curves c
		WHERE NOT EXISTS (SELECT 1 FROM peaks p WHERE p.trial_id = c.trial_id)
		AND c.kind = ?`, model.CurveDSC)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

var _ = time.Now // 保留 time 导入（未来扩展）
