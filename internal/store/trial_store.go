package store

import (
	"database/sql"
	"time"

	"task233-thermopoly/internal/model"
)

// TrialStore 提供试验的持久化操作。
type TrialStore struct{ db *sql.DB }

// NewTrialStore 创建试验仓库。
func NewTrialStore(db *sql.DB) *TrialStore { return &TrialStore{db: db} }

// Insert 写入新试验（幂等：同 ID 冲突报错）。
func (s *TrialStore) Insert(t *model.Trial) error {
	_, err := s.db.Exec(`
		INSERT INTO trials (id, name, material, batch_no, description, status, unit, curve_hash, created_at, updated_at, sealed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Name, t.Material, t.BatchNo, t.Description, t.Status, t.Unit,
		t.CurveHash, ts(t.CreatedAt), ts(t.UpdatedAt), tsPtr(t.SealedAt))
	return err
}

// Get 按 ID 取试验；不存在返回 ErrNotFound。
func (s *TrialStore) Get(id string) (*model.Trial, error) {
	row := s.db.QueryRow(`
		SELECT id, name, material, batch_no, description, status, unit, curve_hash, created_at, updated_at, sealed_at
		FROM trials WHERE id = ?`, id)
	var t model.Trial
	var (
		created, updated string
		sealed           any
	)
	err := row.Scan(&t.ID, &t.Name, &t.Material, &t.BatchNo, &t.Description,
		&t.Status, &t.Unit, &t.CurveHash, &created, &updated, &sealed)
	if err != nil {
		return nil, wrapNotFound(err)
	}
	t.CreatedAt = parseTs(created)
	t.UpdatedAt = parseTs(updated)
	t.SealedAt = parseTsPtr(sealed)
	return &t, nil
}

// List 列出试验（可按状态过滤），按创建时间倒序。
func (s *TrialStore) List(status string, limit int) ([]model.Trial, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	query := `SELECT id, name, material, batch_no, description, status, unit, curve_hash, created_at, updated_at, sealed_at
		FROM trials`
	args := []any{}
	if status != "" {
		query += ` WHERE status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Trial
	for rows.Next() {
		var t model.Trial
		var (
			created, updated string
			sealed           any
		)
		if err := rows.Scan(&t.ID, &t.Name, &t.Material, &t.BatchNo, &t.Description,
			&t.Status, &t.Unit, &t.CurveHash, &created, &updated, &sealed); err != nil {
			return nil, err
		}
		t.CreatedAt = parseTs(created)
		t.UpdatedAt = parseTs(updated)
		t.SealedAt = parseTsPtr(sealed)
		out = append(out, t)
	}
	return out, rows.Err()
}

// UpdateStatus 更新试验状态（状态机校验由 service 层完成，此处仅持久化）。
func (s *TrialStore) UpdateStatus(id, status string, curveHash string) error {
	res, err := s.db.Exec(`UPDATE trials SET status = ?, curve_hash = ?, updated_at = ? WHERE id = ?`,
		status, curveHash, ts(time.Now()), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.ErrNotFound
	}
	return nil
}

// Seal 封存试验：写入封存时间。
func (s *TrialStore) Seal(id string) error {
	now := ts(time.Now())
	res, err := s.db.Exec(`UPDATE trials SET status = ?, sealed_at = ?, updated_at = ? WHERE id = ?`,
		model.TrialSealed, now, now, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.ErrNotFound
	}
	return nil
}

// SetCurveHash 更新试验的曲线内容指纹（用于输入汇总）。
func (s *TrialStore) SetCurveHash(id, hash string) error {
	_, err := s.db.Exec(`UPDATE trials SET curve_hash = ?, updated_at = ? WHERE id = ?`,
		hash, ts(time.Now()), id)
	return err
}

// CountByStatus 统计各状态试验数（供 /api/stats）。
func (s *TrialStore) CountByStatus() (map[string]int, error) {
	rows, err := s.db.Query(`SELECT status, COUNT(*) FROM trials GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var st string
		var n int
		if err := rows.Scan(&st, &n); err != nil {
			return nil, err
		}
		out[st] = n
	}
	return out, rows.Err()
}
