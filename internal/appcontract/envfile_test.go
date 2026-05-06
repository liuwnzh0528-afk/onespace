package appcontract

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadEnvFileParsesSimpleEnvSyntax(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	data := `
# comment
APP_ENV=local
LOG_LEVEL="debug"
TOKEN='abc123'
EMPTY=
export EXPORTED=yes
`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	env, err := ReadEnvFile(path)
	if err != nil {
		t.Fatalf("ReadEnvFile returned error: %v", err)
	}

	want := map[string]string{
		"APP_ENV":   "local",
		"LOG_LEVEL": "debug",
		"TOKEN":     "abc123",
		"EMPTY":     "",
		"EXPORTED":  "yes",
	}
	for key, value := range want {
		if env[key] != value {
			t.Fatalf("env[%s] = %q, want %q", key, env[key], value)
		}
	}
}

func TestReadEnvFileRejectsMalformedLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("not valid\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := ReadEnvFile(path)
	if err == nil {
		t.Fatal("expected malformed line error")
	}
}
