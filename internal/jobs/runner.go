package jobs

import (
	"context"
	"sync"
	"time"
)

type Runner struct {
	Store Store
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func NewRunner(store Store) *Runner {
	return &Runner{
		Store: store,
		locks: make(map[string]*sync.Mutex),
	}
}

func (r *Runner) Run(ctx context.Context, job Job, mutating bool, fn func(context.Context, *Job) error) (Job, error) {
	lockKey := job.Workspace + "/" + job.Service

	if mutating {
		lock := r.getLock(lockKey)
		lock.Lock()
		defer lock.Unlock()
	}

	job.Status = StatusRunning
	job.StartedAt = time.Now()

	if r.Store != nil {
		if err := r.Store.Create(ctx, job); err != nil {
			return job, err
		}
	}

	err := fn(ctx, &job)
	if err != nil {
		job.Status = StatusFailed
	} else {
		job.Status = StatusSuccess
	}
	job.FinishedAt = time.Now()

	if r.Store != nil {
		if updateErr := r.Store.Update(ctx, job); updateErr != nil && err == nil {
			return job, updateErr
		}
	}

	return job, err
}

func (r *Runner) getLock(key string) *sync.Mutex {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.locks[key] == nil {
		r.locks[key] = &sync.Mutex{}
	}
	return r.locks[key]
}
