package jobs

import "context"

type Store interface {
	Create(ctx context.Context, job Job) error
	Update(ctx context.Context, job Job) error
	Get(ctx context.Context, id string) (Job, error)
	List(ctx context.Context, workspace string, limit int) ([]Job, error)
}
