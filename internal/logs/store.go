package logs

import (
	"context"
	"os"
	"path/filepath"
)

type Store struct {
	Root string
}

func (s Store) AppendJob(_ context.Context, jobID string, data []byte) error {
	dir := filepath.Join(s.Root, "jobs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, jobID+".log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

func (s Store) AppendService(_ context.Context, service string, data []byte) error {
	dir := filepath.Join(s.Root, "services")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, service+".log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

func (s Store) ReadJobTail(_ context.Context, jobID string, lines int) ([]string, error) {
	path := filepath.Join(s.Root, "jobs", jobID+".log")
	return tailFile(path, lines)
}

func (s Store) ReadServiceTail(_ context.Context, service string, lines int) ([]string, error) {
	path := filepath.Join(s.Root, "services", service+".log")
	return tailFile(path, lines)
}
