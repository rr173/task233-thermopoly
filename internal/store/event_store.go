package store

import (
	"database/sql"
	"time"

	"task233-thermopoly/internal/model"
)

// EventStore 提供转变事件的持久化操作。
type EventStore struct{ db *sql.DB }

// NewEventStore 创建事件仓库。
func NewEventStore(db *sql.DB) *EventStore { return &EventStore{db: db} }

// InsertMany 批量写入事件候选（单事务）。
func (s *EventStore) InsertMany(events []model.Event) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`
		INSERT INTO events (id, trial_id, peak_id, kind, form, onset_temp, peak_temp,
			confidence, status, evidence, note, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for i := range events {
		e := &events[i]
		if _, err := stmt.Exec(e.ID, e.TrialID, e.PeakID, e.Kind, e.Form,
			e.OnsetTemp, e.PeakTemp, e.Confidence, e.Status, e.Evidence, e.Note,
			ts(e.CreatedAt), ts(e.UpdatedAt)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Get 按 ID 取事件。
func (s *EventStore) Get(id string) (*model.Event, error) {
	row := s.db.QueryRow(`
		SELECT id, trial_id, peak_id, kind, form, onset_temp, peak_temp, confidence,
			status, evidence, note, created_at, updated_at
		FROM events WHERE id = ?`, id)
	var e model.Event
	var (
		created, updated string
		note             sql.NullString
	)
	err := row.Scan(&e.ID, &e.TrialID, &e.PeakID, &e.Kind, &e.Form,
		&e.OnsetTemp, &e.PeakTemp, &e.Confidence, &e.Status, &e.Evidence,
		&note, &created, &updated)
	if err != nil {
		return nil, wrapNotFound(err)
	}
	e.Note = note.String
	e.CreatedAt = parseTs(created)
	e.UpdatedAt = parseTs(updated)
	return &e, nil
}

// ListByTrial 列出某试验的事件（按峰顶温度排序）。
func (s *EventStore) ListByTrial(trialID string) ([]model.Event, error) {
	rows, err := s.db.Query(`
		SELECT e.id, e.trial_id, e.peak_id, e.kind, e.form, e.onset_temp, e.peak_temp, e.confidence,
			e.status, e.evidence, e.note, e.created_at, e.updated_at
		FROM events e JOIN peaks p ON p.id = e.peak_id
		WHERE e.trial_id = ? ORDER BY p.peak_temp`, trialID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEvents(rows)
}

// Update 更新事件（裁决后状态/备注/时间）。
func (s *EventStore) Update(e *model.Event) error {
	res, err := s.db.Exec(`
		UPDATE events SET status = ?, form = ?, confidence = ?, note = ?, updated_at = ?
		WHERE id = ?`,
		e.Status, e.Form, e.Confidence, e.Note, ts(e.UpdatedAt), e.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.ErrNotFound
	}
	return nil
}

// CountByStatus 统计某试验各状态事件数。
func (s *EventStore) CountByStatus(trialID string) (map[string]int, error) {
	rows, err := s.db.Query(`SELECT status, COUNT(*) FROM events WHERE trial_id = ? GROUP BY status`, trialID)
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

func scanEvents(rows *sql.Rows) ([]model.Event, error) {
	var out []model.Event
	for rows.Next() {
		var e model.Event
		var (
			created, updated string
			note             sql.NullString
		)
		if err := rows.Scan(&e.ID, &e.TrialID, &e.PeakID, &e.Kind, &e.Form,
			&e.OnsetTemp, &e.PeakTemp, &e.Confidence, &e.Status, &e.Evidence,
			&note, &created, &updated); err != nil {
			return nil, err
		}
		e.Note = note.String
		e.CreatedAt = parseTs(created)
		e.UpdatedAt = parseTs(updated)
		out = append(out, e)
	}
	return out, rows.Err()
}

var _ = time.Now
