package store

import (
	"database/sql"

	"task233-thermopoly/internal/model"
)

// SnapshotStore 提供判读快照的持久化操作。
type SnapshotStore struct{ db *sql.DB }

// NewSnapshotStore 创建快照仓库。
func NewSnapshotStore(db *sql.DB) *SnapshotStore { return &SnapshotStore{db: db} }

// Insert 写入快照。
func (s *SnapshotStore) Insert(sn *model.Snapshot) error {
	_, err := s.db.Exec(`
		INSERT INTO snapshots (id, trial_id, version, status, summary, event_ids, frozen_inputs, published_at, replaced_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sn.ID, sn.TrialID, sn.Version, sn.Status, sn.Summary,
		jsonString(sn.EventIDs), sn.FrozenInputs, tsPtr(sn.PublishedAt), sn.ReplacedBy, ts(sn.CreatedAt))
	return err
}

// Get 按 ID 取快照。
func (s *SnapshotStore) Get(id string) (*model.Snapshot, error) {
	row := s.db.QueryRow(`
		SELECT id, trial_id, version, status, summary, event_ids, frozen_inputs, published_at, replaced_by, created_at
		FROM snapshots WHERE id = ?`, id)
	return scanSnapshot(row)
}

// GetByTrialVersion 按试验与版本号取快照。
func (s *SnapshotStore) GetByTrialVersion(trialID string, version int) (*model.Snapshot, error) {
	row := s.db.QueryRow(`
		SELECT id, trial_id, version, status, summary, event_ids, frozen_inputs, published_at, replaced_by, created_at
		FROM snapshots WHERE trial_id = ? AND version = ?`, trialID, version)
	return scanSnapshot(row)
}

// ListByTrial 列出某试验的全部快照（版本倒序）。
func (s *SnapshotStore) ListByTrial(trialID string) ([]model.Snapshot, error) {
	rows, err := s.db.Query(`
		SELECT id, trial_id, version, status, summary, event_ids, frozen_inputs, published_at, replaced_by, created_at
		FROM snapshots WHERE trial_id = ? ORDER BY version DESC`, trialID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Snapshot
	for rows.Next() {
		sn, err := scanSnapshotRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *sn)
	}
	return out, rows.Err()
}

// NextVersion 返回某试验的下一个快照版本号。
func (s *SnapshotStore) NextVersion(trialID string) (int, error) {
	var n sql.NullInt64
	err := s.db.QueryRow(`SELECT MAX(version) FROM snapshots WHERE trial_id = ?`, trialID).Scan(&n)
	if err != nil {
		return 0, err
	}
	if !n.Valid {
		return 1, nil
	}
	return int(n.Int64) + 1, nil
}

// Update 更新快照（发布/替代后的状态与时间字段）。
func (s *SnapshotStore) Update(sn *model.Snapshot) error {
	res, err := s.db.Exec(`
		UPDATE snapshots SET status = ?, summary = ?, event_ids = ?, frozen_inputs = ?, published_at = ?, replaced_by = ?
		WHERE id = ?`,
		sn.Status, sn.Summary, jsonString(sn.EventIDs), sn.FrozenInputs,
		tsPtr(sn.PublishedAt), sn.ReplacedBy, sn.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.ErrNotFound
	}
	return nil
}

// Published 取某试验当前已发布的快照（最新发布版本，仅状态为 published；
// 草稿/已替代版本不算当前发布结果。若无返回 ErrNotFound）。
func (s *SnapshotStore) Published(trialID string) (*model.Snapshot, error) {
	row := s.db.QueryRow(`
		SELECT id, trial_id, version, status, summary, event_ids, frozen_inputs, published_at, replaced_by, created_at
		FROM snapshots WHERE trial_id = ? AND status = ?
		ORDER BY version DESC LIMIT 1`,
		trialID, model.SnapshotPublished)
	return scanSnapshot(row)
}

func scanSnapshot(row *sql.Row) (*model.Snapshot, error) {
	var sn model.Snapshot
	var (
		eventIDs    string
		created     string
		published   any
	)
	err := row.Scan(&sn.ID, &sn.TrialID, &sn.Version, &sn.Status, &sn.Summary,
		&eventIDs, &sn.FrozenInputs, &published, &sn.ReplacedBy, &created)
	if err != nil {
		return nil, wrapNotFound(err)
	}
	if err := jsonUnmarshal(eventIDs, &sn.EventIDs); err != nil {
		return nil, err
	}
	sn.CreatedAt = parseTs(created)
	sn.PublishedAt = parseTsPtr(published)
	return &sn, nil
}

func scanSnapshotRow(rows *sql.Rows) (*model.Snapshot, error) {
	var sn model.Snapshot
	var (
		eventIDs  string
		created   string
		published any
	)
	err := rows.Scan(&sn.ID, &sn.TrialID, &sn.Version, &sn.Status, &sn.Summary,
		&eventIDs, &sn.FrozenInputs, &published, &sn.ReplacedBy, &created)
	if err != nil {
		return nil, err
	}
	if err := jsonUnmarshal(eventIDs, &sn.EventIDs); err != nil {
		return nil, err
	}
	sn.CreatedAt = parseTs(created)
	sn.PublishedAt = parseTsPtr(published)
	return &sn, nil
}
