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
	DBPath           string
	DatabaseURL      string
	Port             string
	ScanPaths        []string
	RescanInterval   time.Duration
	MetadataMaxTime  time.Duration
	LogLevel         string
	CacheDir         string
	CacheMaxBytes    int64
	ScanWorkers      int
	UpstreamURLs     []string
	ZabbixKey        string
	EnvFile          string
}

// Load читает .env, переменные окружения и флаги командной строки.
func Load() Config {
	_ = godotenv.Load()

	cfg := Config{
		DBPath:          envOr("DEBUGINFOD_DB_PATH", "debuginfod.sqlite"),
		DatabaseURL:     envOr("DEBUGINFOD_DATABASE_URL", ""),
		Port:            envOr("DEBUGINFOD_PORT", "8002"),
		ScanPaths:       splitPaths(envOr("DEBUGINFOD_SCAN_PATH", ".")),
		RescanInterval:  envDuration("DEBUGINFOD_RESCAN_INTERVAL", time.Hour),
		MetadataMaxTime: envDuration("DEBUGINFOD_METADATA_MAXTIME", 5*time.Second),
		LogLevel:        envOr("DEBUGINFOD_LOG_LEVEL", "info"),
		CacheDir:        envOr("DEBUGINFOD_CACHE_DIR", ".debuginfod-cache"),
		CacheMaxBytes:   envInt64("DEBUGINFOD_CACHE_MAX_BYTES", 0),
		ScanWorkers:     envInt("DEBUGINFOD_SCAN_WORKERS", 4),
		UpstreamURLs:    splitPaths(envOr("DEBUGINFOD_URLS", "")),
		ZabbixKey:       envOr("DEBUGINFOD_ZABBIX_KEY", ""),
		EnvFile:         envOr("DEBUGINFOD_ENV_FILE", ".env"),
	}

	if cfg.EnvFile != "" && cfg.EnvFile != ".env" {
		_ = godotenv.Load(cfg.EnvFile)
	}

	flag.StringVar(&cfg.DBPath, "d", cfg.DBPath, "путь к SQLite базе данных")
	flag.StringVar(&cfg.DatabaseURL, "database-url", cfg.DatabaseURL, "PostgreSQL URL (postgres://...)")
	flag.StringVar(&cfg.Port, "p", cfg.Port, "порт для HTTP-сервера")
	scanPath := strings.Join(cfg.ScanPaths, ",")
	flag.StringVar(&scanPath, "s", scanPath, "пути для сканирования (через запятую)")
	flag.DurationVar(&cfg.RescanInterval, "r", cfg.RescanInterval, "интервал переиндексации")
	flag.DurationVar(&cfg.MetadataMaxTime, "metadata-maxtime", cfg.MetadataMaxTime, "лимит metadata-запросов")
	flag.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "уровень логирования")
	flag.StringVar(&cfg.CacheDir, "cache", cfg.CacheDir, "каталог кэша")
	flag.Int64Var(&cfg.CacheMaxBytes, "cache-max-bytes", cfg.CacheMaxBytes, "лимит кэша в байтах (0=без лимита)")
	flag.IntVar(&cfg.ScanWorkers, "scan-workers", cfg.ScanWorkers, "число параллельных воркеров индексации")
	upstreams := strings.Join(cfg.UpstreamURLs, ",")
	flag.StringVar(&upstreams, "upstream", upstreams, "upstream debuginfod URLs для федерации")
	flag.StringVar(&cfg.ZabbixKey, "zabbix-key", cfg.ZabbixKey, "токен для /zabbix endpoint")
	flag.StringVar(&cfg.EnvFile, "env-file", cfg.EnvFile, "путь к .env")
	flag.Parse()

	cfg.ScanPaths = splitPaths(scanPath)
	cfg.UpstreamURLs = splitPaths(upstreams)
	if len(cfg.ScanPaths) == 0 {
		cfg.ScanPaths = []string{"."}
	}
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

func envInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return n
}

func envInt64(key string, fallback int64) int64 {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fallback
	}
	return n
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
	if len(out) == 0 && raw == "." {
		return []string{"."}
	}
	return out
}

func (c Config) PortInt() (int, error) {
	return strconv.Atoi(c.Port)
}
