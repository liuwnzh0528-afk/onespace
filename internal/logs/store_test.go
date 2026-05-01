package logs

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLogStoreAppendsAndTailsLines(t *testing.T) {
	dir := t.TempDir()
	s := Store{Root: dir}
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		if err := s.AppendJob(ctx, "job1", []byte("line "+string(rune('0'+i))+"\n")); err != nil {
			t.Fatal(err)
		}
	}

	lines, err := s.ReadJobTail(ctx, "job1", 3)
	if err != nil {
		t.Fatalf("ReadJobTail: %v", err)
	}
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}
	if lines[0] != "line 2" {
		t.Fatalf("first line = %q, want %q", lines[0], "line 2")
	}
	if lines[2] != "line 4" {
		t.Fatalf("last line = %q, want %q", lines[2], "line 4")
	}

	// Service logs
	if err := s.AppendService(ctx, "user-api", []byte("service log line\n")); err != nil {
		t.Fatal(err)
	}
	lines, err = s.ReadServiceTail(ctx, "user-api", 10)
	if err != nil {
		t.Fatalf("ReadServiceTail: %v", err)
	}
	if len(lines) != 1 || lines[0] != "service log line" {
		t.Fatalf("unexpected service lines: %v", lines)
	}
}

func TestLogStoreReturnsEmptyTailForMissingLog(t *testing.T) {
	dir := t.TempDir()
	s := Store{Root: dir}
	ctx := context.Background()

	lines, err := s.ReadJobTail(ctx, "nonexistent", 10)
	if err != nil {
		t.Fatalf("ReadJobTail for missing: %v", err)
	}
	if lines != nil {
		t.Fatalf("expected nil for missing log, got %v", lines)
	}

	// Ensure the log directory doesn't exist either
	lines, err = s.ReadServiceTail(ctx, "nonexistent", 10)
	if err != nil {
		t.Fatalf("ReadServiceTail for missing: %v", err)
	}
	if lines != nil {
		t.Fatalf("expected nil for missing service log, got %v", lines)
	}
}

func TestTailFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	content := ""
	for i := 0; i < 100; i++ {
		content += "line " + string(rune('0'+i%10)) + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	lines, err := tailFile(path, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 5 {
		t.Fatalf("got %d lines, want 5", len(lines))
	}
}
