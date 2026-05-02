package jobs

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunnerSerializesMutatingJobsPerService(t *testing.T) {
	runner := NewRunner(nil)

	var running int32
	started := make(chan struct{})
	release := make(chan struct{})

	job1 := Job{ID: "1", Type: TypeDeploy, Workspace: "ws", Service: "user-api"}
	job2 := Job{ID: "2", Type: TypeDeploy, Workspace: "ws", Service: "user-api"}

	go func() {
		_, _ = runner.Run(context.Background(), job1, true, func(ctx context.Context, j *Job) error {
			atomic.AddInt32(&running, 1)
			close(started)
			<-release
			atomic.AddInt32(&running, -1)
			return nil
		})
	}()

	<-started

	done := make(chan struct{})
	go func() {
		_, _ = runner.Run(context.Background(), job2, true, func(ctx context.Context, j *Job) error {
			atomic.AddInt32(&running, 1)
			atomic.AddInt32(&running, -1)
			return nil
		})
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	if atomic.LoadInt32(&running) != 1 {
		t.Fatal("second mutating job should not start while first is running")
	}

	close(release)
	<-done

	if atomic.LoadInt32(&running) != 0 {
		t.Fatal("both jobs should be done")
	}
}

func TestRunnerAllowsReadOnlyJobsDuringMutatingJob(t *testing.T) {
	runner := NewRunner(nil)

	started := make(chan struct{})
	release := make(chan struct{})

	job1 := Job{ID: "1", Type: TypeDeploy, Workspace: "ws", Service: "user-api"}
	job2 := Job{ID: "2", Type: TypeHealth, Workspace: "ws", Service: "user-api"}

	go func() {
		_, _ = runner.Run(context.Background(), job1, true, func(ctx context.Context, j *Job) error {
			close(started)
			<-release
			return nil
		})
	}()

	<-started

	var readRan bool
	_, err := runner.Run(context.Background(), job2, false, func(ctx context.Context, j *Job) error {
		readRan = true
		return nil
	})
	close(release)

	if err != nil {
		t.Fatalf("read-only job failed: %v", err)
	}
	if !readRan {
		t.Fatal("read-only job should run concurrently with mutating job")
	}
}

func TestSQLiteStorePersistsJobResult(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := OpenSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLiteStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	job := Job{
		ID:        "job_test_001",
		Type:      TypeDeploy,
		Workspace: "ws",
		Service:   "user-api",
		Status:    StatusSuccess,
		Stage:     "done",
		StartedAt: time.Now().Truncate(time.Second),
		Result:    []byte(`{"deployed":true}`),
	}

	if err := store.Create(ctx, job); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := store.Get(ctx, job.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if got.ID != job.ID {
		t.Fatalf("ID = %q, want %q", got.ID, job.ID)
	}
	if got.Type != job.Type {
		t.Fatalf("Type = %q, want %q", got.Type, job.Type)
	}
	if got.Status != job.Status {
		t.Fatalf("Status = %q, want %q", got.Status, job.Status)
	}
	if string(got.Result) != string(job.Result) {
		t.Fatalf("Result = %q, want %q", got.Result, job.Result)
	}
}

func TestRunnerPersistsSQLiteJobLifecycle(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := OpenSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLiteStore: %v", err)
	}
	defer store.Close()

	runner := NewRunner(store)
	job := Job{
		ID:        "job_runner_sqlite_001",
		Type:      TypeDeploy,
		Workspace: "ws",
		Service:   "user-api",
	}

	_, err = runner.Run(context.Background(), job, true, func(ctx context.Context, j *Job) error {
		j.Stage = "done"
		j.Result = []byte(`{"status":"success"}`)
		return nil
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	jobs, err := store.List(context.Background(), "ws", 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("stored jobs = %d, want 1", len(jobs))
	}
	if jobs[0].Status != StatusSuccess || jobs[0].Stage != "done" {
		t.Fatalf("stored job = %+v", jobs[0])
	}
}
