package config

import (
	"flag"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config содержит все параметры запуска debuginfod.
type Config struct {
	DBPath         string
	Port           string
	ScanPaths      []string
	RescanInterval time.Duration
	LogLevel       string
	CacheDir       string
	EnvFile        string
}

// Load читает .env, переменные окружения и флаги командной строки.
// Приоритет: флаги > env > .env > значения по умолчанию.
func Load() Config {
	_ = godotenv.Load()

	cfg := Config{
		DBPath:         envOr("DEBUGINFOD_DB_PATH", "debuginfod.sqlite"),
		Port:           envOr("DEBUGINFOD_PORT", "8002"),
		ScanPaths:      splitPaths(envOr("DEBUGINFOD_SCAN_PATH", ".")),
		RescanInterval: envDuration("DEBUGINFOD_RESCAN_INTERVAL", time.Hour),
		LogLevel:       envOr("DEBUGINFOD_LOG_LEVEL", "info"),
		CacheDir:       envOr("DEBUGINFOD_CACHE_DIR", ".debuginfod-cache"),
		EnvFile:        envOr("DEBUGINFOD_ENV_FILE", ".env"),
	}

	if cfg.EnvFile != "" && cfg.EnvFile != ".env" {
		_ = godotenv.Load(cfg.EnvFile)
	}

	flag.StringVar(&cfg.DBPath, "d", cfg.DBPath, "путь к SQLite базе данных")
	flag.StringVar(&cfg.Port, "p", cfg.Port, "порт для HTTP-сервера")
	scanPath := strings.Join(cfg.ScanPaths, ",")
	flag.StringVar(&scanPath, "s", scanPath, "пути для сканирования (через запятую)")
	flag.DurationVar(&cfg.RescanInterval, "r", cfg.RescanInterval, "интервал переиндексации")
	flag.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "уровень логирования")
	flag.StringVar(&cfg.CacheDir, "cache", cfg.CacheDir, "каталог кэша извлечённых файлов")
	flag.StringVar(&cfg.EnvFile, "env-file", cfg.EnvFile, "путь к .env файлу")
	flag.Parse()

	cfg.ScanPaths = splitPaths(scanPath)
	return cfg
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return d
}

func splitPaths(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		return []string{"."}
	}
	return out
}

// PortInt возвращает порт как число (для валидации).
func (c Config) PortInt() (int, error) {
	return strconv.Atoi(c.Port)
}
