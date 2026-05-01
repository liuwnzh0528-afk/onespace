package version

import "testing"

func TestInfoUsesDevDefaults(t *testing.T) {
	info := Info()
	if info.Name != "onespace" {
		t.Fatalf("Name = %q, want onespace", info.Name)
	}
	if info.Version != "dev" {
		t.Fatalf("Version = %q, want dev", info.Version)
	}
	if info.Commit != "none" {
		t.Fatalf("Commit = %q, want none", info.Commit)
	}
}
