package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

func OpenSQLiteStore(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	s := &SQLiteStore{db: db}
	if err := s.init(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) init() error {
	schema := `
	CREATE TABLE IF NOT EXISTS jobs (
		id TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		workspace TEXT NOT NULL,
		service TEXT NOT NULL,
		status TEXT NOT NULL,
		stage TEXT NOT NULL DEFAULT '',
		started_at DATETIME NOT NULL,
		finished_at DATETIME NOT NULL DEFAULT '',
		exit_code INTEGER NOT NULL DEFAULT 0,
		log_ref TEXT NOT NULL DEFAULT '',
		result BLOB NOT NULL DEFAULT ''
	);`
	_, err := s.db.Exec(schema)
	return err
}

func (s *SQLiteStore) Create(ctx context.Context, job Job) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO jobs (id, type, workspace, service, status, stage, started_at, finished_at, exit_code, log_ref, result)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID, job.Type, job.Workspace, job.Service, job.Status, job.Stage,
		job.StartedAt, job.FinishedAt, job.ExitCode, job.LogRef, job.Result,
	)
	return err
}

func (s *SQLiteStore) Update(ctx context.Context, job Job) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE jobs SET type=?, workspace=?, service=?, status=?, stage=?, started_at=?, finished_at=?, exit_code=?, log_ref=?, result=? WHERE id=?`,
		job.Type, job.Workspace, job.Service, job.Status, job.Stage,
		job.StartedAt, job.FinishedAt, job.ExitCode, job.LogRef, job.Result, job.ID,
	)
	return err
}

func (s *SQLiteStore) Get(ctx context.Context, id string) (Job, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, type, workspace, service, status, stage, started_at, finished_at, exit_code, log_ref, result FROM jobs WHERE id=?`,
		id,
	)
	var job Job
	var startedAt, finishedAt string
	var exitCode int
	var logRef string
	var result []byte

	err := row.Scan(&job.ID, &job.Type, &job.Workspace, &job.Service, &job.Status, &job.Stage,
		&startedAt, &finishedAt, &exitCode, &logRef, &result)
	if err != nil {
		return Job{}, err
	}

	job.StartedAt, _ = time.Parse(time.RFC3339, startedAt)
	job.FinishedAt, _ = time.Parse(time.RFC3339, finishedAt)
	job.ExitCode = exitCode
	job.LogRef = logRef
	job.Result = result
	return job, nil
}

func (s *SQLiteStore) List(ctx context.Context, workspace string, limit int) ([]Job, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, type, workspace, service, status, stage, started_at, finished_at, exit_code, log_ref, result
		 FROM jobs WHERE workspace=? ORDER BY started_at DESC LIMIT ?`,
		workspace, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var job Job
		var startedAt, finishedAt string
		var exitCode int
		var logRef string
		var result []byte

		err := rows.Scan(&job.ID, &job.Type, &job.Workspace, &job.Service, &job.Status, &job.Stage,
			&startedAt, &finishedAt, &exitCode, &logRef, &result)
		if err != nil {
			return nil, err
		}
		job.StartedAt, _ = time.Parse(time.RFC3339, startedAt)
		job.FinishedAt, _ = time.Parse(time.RFC3339, finishedAt)
		job.ExitCode = exitCode
		job.LogRef = logRef
		job.Result = result
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}
