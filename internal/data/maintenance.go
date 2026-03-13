package data

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

type MaintenanceOptions struct {
	RetentionDays  int
	ArchiveEnabled bool
	ArchivePath    string
	BatchSize      int
}

type archivedEventRecord struct {
	ID       int64
	TimeUnix int64
	DeviceID int
	Code     string
	Type     string
	Desc     string
	Zone     string
	GroupNo  string
	Priority int
	Category string
	Archived int64
}

func (r *Repository) RunMaintenance(ctx context.Context, opts MaintenanceOptions) error {
	r.writeLock.Lock()
	defer r.writeLock.Unlock()

	if opts.BatchSize <= 0 {
		opts.BatchSize = 5000
	}
	if opts.RetentionDays > 0 {
		if err := r.archiveAndDeleteOldEvents(ctx, opts); err != nil {
			return err
		}
	}
	return r.compactAndVacuumLocked(ctx)
}

func (r *Repository) archiveAndDeleteOldEvents(ctx context.Context, opts MaintenanceOptions) error {
	cutoff := time.Now().AddDate(0, 0, -opts.RetentionDays).Unix()
	if cutoff <= 0 {
		return nil
	}
	archiveEnabled := opts.ArchiveEnabled && strings.TrimSpace(opts.ArchivePath) != ""
	totalArchived := 0
	totalDeleted := 0
	for {
		batch, err := r.loadArchivedBatch(ctx, cutoff, opts.BatchSize)
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			break
		}
		if archiveEnabled {
			if err := archiveEventBatch(ctx, opts.ArchivePath, batch); err != nil {
				return err
			}
			totalArchived += len(batch)
		}
		if err := r.deleteEventBatchLocked(ctx, batch); err != nil {
			return err
		}
		totalDeleted += len(batch)
		if len(batch) < opts.BatchSize {
			break
		}
	}
	if totalDeleted > 0 {
		log.Info().
			Int("deleted", totalDeleted).
			Int("archived", totalArchived).
			Int("retention_days", opts.RetentionDays).
			Msg("history retention maintenance completed")
	}
	return nil
}

func (r *Repository) loadArchivedBatch(ctx context.Context, cutoff int64, limit int) ([]archivedEventRecord, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT
	e.id,
	COALESCE(e.time, 0),
	COALESCE(d.number, 0),
	COALESCE(c.code, ''),
	COALESCE(c.type_text, ''),
	COALESCE(c.desc_text, ''),
	COALESCE(z.zone_text, ''),
	COALESCE(e.group_no, ''),
	COALESCE(e.priority, 0),
	COALESCE(et.key, 'other')
FROM events e
JOIN devices d ON d.id = e.device_ref_id
JOIN event_catalog c ON c.id = e.event_catalog_id
JOIN event_types et ON et.id = e.event_type_id
LEFT JOIN event_zones z ON z.id = e.zone_ref_id
WHERE e.time < ?
ORDER BY e.time ASC, e.id ASC
LIMIT ?`, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	archivedAt := time.Now().Unix()
	out := make([]archivedEventRecord, 0, limit)
	for rows.Next() {
		var row archivedEventRecord
		row.Archived = archivedAt
		if err := rows.Scan(
			&row.ID,
			&row.TimeUnix,
			&row.DeviceID,
			&row.Code,
			&row.Type,
			&row.Desc,
			&row.Zone,
			&row.GroupNo,
			&row.Priority,
			&row.Category,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *Repository) deleteEventBatchLocked(ctx context.Context, batch []archivedEventRecord) error {
	if len(batch) == 0 {
		return nil
	}
	args := make([]any, 0, len(batch))
	holders := make([]string, 0, len(batch))
	for _, item := range batch {
		args = append(args, item.ID)
		holders = append(holders, "?")
	}
	_, err := r.db.ExecContext(ctx, "DELETE FROM events WHERE id IN ("+strings.Join(holders, ",")+")", args...)
	return err
}

func archiveEventBatch(ctx context.Context, archivePath string, batch []archivedEventRecord) error {
	if len(batch) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
		return err
	}
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000", archivePath))
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS archived_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	source_event_id INTEGER,
	time INTEGER NOT NULL,
	device_number INTEGER NOT NULL,
	code TEXT NOT NULL,
	type_text TEXT,
	desc_text TEXT,
	zone_text TEXT,
	group_no TEXT,
	priority INTEGER,
	category TEXT,
	archived_at INTEGER NOT NULL
)`); err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO archived_events (
	source_event_id, time, device_number, code, type_text, desc_text,
	zone_text, group_no, priority, category, archived_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, item := range batch {
		if _, err := stmt.ExecContext(
			ctx,
			item.ID,
			item.TimeUnix,
			item.DeviceID,
			item.Code,
			item.Type,
			item.Desc,
			item.Zone,
			item.GroupNo,
			item.Priority,
			item.Category,
			item.Archived,
		); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}
