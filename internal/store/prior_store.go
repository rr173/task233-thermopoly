package store

import (
	"database/sql"
	"time"

	"task233-thermopoly/internal/model"
)

// PriorStore 提供晶型先验知识的持久化操作。
type PriorStore struct{ db *sql.DB }

// NewPriorStore 创建先验仓库。
func NewPriorStore(db *sql.DB) *PriorStore { return &PriorStore{db: db} }

// Insert 写入先验。
func (s *PriorStore) Insert(p *model.PolymorphPrior) error {
	_, err := s.db.Exec(`
		INSERT INTO priors (id, form_from, form_to, onset_low, onset_high, direction, max_mass_loss_pct, note, active, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.FormFrom, p.FormTo, p.OnsetLow, p.OnsetHigh, p.Direction,
		p.MaxMassLossPct, p.Note, boolToInt(p.Active), ts(p.CreatedAt))
	return err
}

// Get 按 ID 取先验。
func (s *PriorStore) Get(id string) (*model.PolymorphPrior, error) {
	row := s.db.QueryRow(`
		SELECT id, form_from, form_to, onset_low, onset_high, direction, max_mass_loss_pct, note, active, created_at
		FROM priors WHERE id = ?`, id)
	var p model.PolymorphPrior
	var created string
	err := row.Scan(&p.ID, &p.FormFrom, &p.FormTo, &p.OnsetLow, &p.OnsetHigh,
		&p.Direction, &p.MaxMassLossPct, &p.Note, &p.Active, &created)
	if err != nil {
		return nil, wrapNotFound(err)
	}
	p.CreatedAt = parseTs(created)
	return &p, nil
}

// List 列出全部先验（按 onset_low 排序）。
func (s *PriorStore) List() ([]model.PolymorphPrior, error) {
	rows, err := s.db.Query(`
		SELECT id, form_from, form_to, onset_low, onset_high, direction, max_mass_loss_pct, note, active, created_at
		FROM priors ORDER BY onset_low`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.PolymorphPrior
	for rows.Next() {
		var p model.PolymorphPrior
		var created string
		if err := rows.Scan(&p.ID, &p.FormFrom, &p.FormTo, &p.OnsetLow, &p.OnsetHigh,
			&p.Direction, &p.MaxMassLossPct, &p.Note, &p.Active, &created); err != nil {
			return nil, err
		}
		p.CreatedAt = parseTs(created)
		out = append(out, p)
	}
	return out, rows.Err()
}

// ActiveList 列出启用的先验（事件分类只使用启用项）。
func (s *PriorStore) ActiveList() ([]model.PolymorphPrior, error) {
	rows, err := s.db.Query(`
		SELECT id, form_from, form_to, onset_low, onset_high, direction, max_mass_loss_pct, note, active, created_at
		FROM priors WHERE active = 1 ORDER BY onset_low`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.PolymorphPrior
	for rows.Next() {
		var p model.PolymorphPrior
		var created string
		if err := rows.Scan(&p.ID, &p.FormFrom, &p.FormTo, &p.OnsetLow, &p.OnsetHigh,
			&p.Direction, &p.MaxMassLossPct, &p.Note, &p.Active, &created); err != nil {
			return nil, err
		}
		p.CreatedAt = parseTs(created)
		out = append(out, p)
	}
	return out, rows.Err()
}

// Update 更新先验（含启用/停用）。
func (s *PriorStore) Update(p *model.PolymorphPrior) error {
	res, err := s.db.Exec(`
		UPDATE priors SET form_from = ?, form_to = ?, onset_low = ?, onset_high = ?,
			direction = ?, max_mass_loss_pct = ?, note = ?, active = ?
		WHERE id = ?`,
		p.FormFrom, p.FormTo, p.OnsetLow, p.OnsetHigh, p.Direction,
		p.MaxMassLossPct, p.Note, boolToInt(p.Active), p.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.ErrNotFound
	}
	return nil
}

// Count 统计先验总数。
func (s *PriorStore) Count() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM priors`).Scan(&n)
	return n, err
}

var _ = time.Now
