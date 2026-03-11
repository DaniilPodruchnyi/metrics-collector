package config

import (
	"flag"
	"os"
	"testing"
)

func TestParseServerConfig_EnvOverridesDefaults(t *testing.T) {
	t.Setenv("ADDRESS", "127.0.0.1:9999")
	t.Setenv("STORE_INTERVAL", "1")
	t.Setenv("FILE_STORAGE_PATH", "file.json")
	t.Setenv("RESTORE", "false")
	t.Setenv("AUDIT_FILE", "audit.log")
	t.Setenv("AUDIT_URL", "http://localhost:8081/audit")

	// ParseServerConfig использует глобальный flag.CommandLine.
	// Чтобы не пересекаться с флагами других тестов, пересоздаём FlagSet.
	old := flag.CommandLine
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	t.Cleanup(func() { flag.CommandLine = old })

	cfg, err := ParseServerConfig()
	if err != nil {
		t.Fatalf("ParseServerConfig failed: %v", err)
	}

	if cfg.Address != "127.0.0.1:9999" {
		t.Fatalf("Address: %q", cfg.Address)
	}
	if cfg.FileStoragePath != "file.json" {
		t.Fatalf("FileStoragePath: %q", cfg.FileStoragePath)
	}
	if cfg.Restore != false {
		t.Fatalf("Restore: expected false")
	}
	if cfg.AuditFile != "audit.log" {
		t.Fatalf("AuditFile: %q", cfg.AuditFile)
	}
	if cfg.AuditURL != "http://localhost:8081/audit" {
		t.Fatalf("AuditURL: %q", cfg.AuditURL)
	}
}
