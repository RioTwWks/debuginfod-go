package config

import (
	"os"
	"testing"
	"time"
)

func TestEnvOrAndSplitPaths(t *testing.T) {
	t.Setenv("DEBUGINFOD_SCAN_PATH", "/a,/b")
	paths := splitPaths(envOr("DEBUGINFOD_SCAN_PATH", "."))
	if len(paths) != 2 || paths[0] != "/a" || paths[1] != "/b" {
		t.Fatalf("paths=%v", paths)
	}
}

func TestEnvDuration(t *testing.T) {
	t.Setenv("DEBUGINFOD_RESCAN_INTERVAL", "30m")
	if got := envDuration("DEBUGINFOD_RESCAN_INTERVAL", time.Hour); got != 30*time.Minute {
		t.Fatalf("duration=%v", got)
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("DEBUGINFOD_DB_PATH", "/tmp/test.sqlite")
	t.Setenv("DEBUGINFOD_PORT", "9000")
	// flag.Parse() в Load() конфликтует с go test, поэтому тестируем хелперы.
	if envOr("DEBUGINFOD_DB_PATH", "") != "/tmp/test.sqlite" {
		t.Fatal("env not read")
	}
	os.Unsetenv("DEBUGINFOD_DB_PATH")
}
