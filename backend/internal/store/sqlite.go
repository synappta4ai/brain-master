// Package store implementa la capa de persistencia del gateway.
// SQLite puro-Go (modernc.org/sqlite): sin CGo, binario estático.
package store

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite"

	"brain-master/backend/internal/queue"
)

// SQLiteStore persiste los jobs en brain-master.db y sobrevive a reinicios.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore abre (o crea) la base y aplica el esquema.
func NewSQLiteStore(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("abriendo sqlite: %w", err)
	}
	// modernc/sqlite maneja una conexión a la vez sin problemas; serializa escrituras.
	db.SetMaxOpenConns(1)

	schema := `
	CREATE TABLE IF NOT EXISTS jobs (
		id              TEXT PRIMARY KEY,
		mode            TEXT    NOT NULL DEFAULT 'video',
		prompt          TEXT    NOT NULL DEFAULT '',
		negative_prompt TEXT    NOT NULL DEFAULT '',
		model           TEXT    NOT NULL DEFAULT '',
		width           INTEGER NOT NULL DEFAULT 0,
		height          INTEGER NOT NULL DEFAULT 0,
		frames          INTEGER NOT NULL DEFAULT 0,
		fps             INTEGER NOT NULL DEFAULT 0,
		seed            INTEGER NOT NULL DEFAULT 0,
		status          TEXT    NOT NULL DEFAULT 'QUEUED',
		progress        REAL    NOT NULL DEFAULT 0,
		output_path     TEXT    NOT NULL DEFAULT '',
		step            INTEGER NOT NULL DEFAULT 0,
		created_at      INTEGER NOT NULL DEFAULT 0,
		completed_at    INTEGER
	);
	CREATE INDEX IF NOT EXISTS idx_jobs_step ON jobs(step);
	`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("aplicando esquema: %w", err)
	}

	log.Printf("[Store] SQLite listo en %s", path)
	return &SQLiteStore{db: db}, nil
}

// Close cierra la conexión.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// UpsertJob inserta o actualiza un job.
func (s *SQLiteStore) UpsertJob(job *queue.GenerationJob) error {
	if s == nil || s.db == nil {
		return nil
	}
	var completedAt any
	if job.CompletedAt != nil {
		completedAt = job.CompletedAt.UnixNano()
	}
	_, err := s.db.Exec(`
		INSERT INTO jobs (id, mode, prompt, negative_prompt, model, width, height,
			frames, fps, seed, status, progress, output_path, step, created_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			status = excluded.status,
			progress = excluded.progress,
			output_path = excluded.output_path,
			completed_at = excluded.completed_at`,
		job.ID, job.Mode, job.Prompt, job.NegativePrompt, job.Model,
		job.Width, job.Height, job.Frames, job.Fps, job.Seed,
		string(job.Status), job.Progress, job.OutputPath, job.Step,
		job.CreatedAt.UnixNano(), completedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert job %s: %w", job.ID, err)
	}
	return nil
}

// LoadAll devuelve todos los jobs ordenados por creación (step de persistencia).
func (s *SQLiteStore) LoadAll() ([]*queue.GenerationJob, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	rows, err := s.db.Query(`
		SELECT id, mode, prompt, negative_prompt, model, width, height,
		       frames, fps, seed, status, progress, output_path, step,
		       created_at, completed_at
		FROM jobs ORDER BY step ASC`)
	if err != nil {
		return nil, fmt.Errorf("loadall: %w", err)
	}
	defer rows.Close()

	jobs := []*queue.GenerationJob{}
	for rows.Next() {
		var (
			j           queue.GenerationJob
			status      string
			createdAtNs int64
			completedAt sql.NullInt64
		)
		if err := rows.Scan(&j.ID, &j.Mode, &j.Prompt, &j.NegativePrompt, &j.Model,
			&j.Width, &j.Height, &j.Frames, &j.Fps, &j.Seed,
			&status, &j.Progress, &j.OutputPath, &j.Step,
			&createdAtNs, &completedAt); err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}
		j.Status = queue.JobStatus(status)
		j.CreatedAt = time.Unix(0, createdAtNs)
		if completedAt.Valid {
			t := time.Unix(0, completedAt.Int64)
			j.CompletedAt = &t
		}
		jobs = append(jobs, &j)
	}
	return jobs, rows.Err()
}
