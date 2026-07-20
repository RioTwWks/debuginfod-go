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
	LazyExtract      bool
	UIEnabled        bool
	CORSOrigins      []string
	RateLimitRPS     float64
	BasicAuthUser    string
	BasicAuthPass    string
	TLSCertFile      string
	TLSKeyFile       string
	TLSClientCA      string
	MetadataPageSize int
	ScanEnabled      bool
	AdminKey         string
	ScanWebhookURL   string
	Dedup            DedupConfig
}

// DedupConfig — параметры xdelta dedup для Quik .debug.
type DedupConfig struct {
	Enabled      bool
	Projects     []string
	Workers      int
	BlobDir      string // legacy zstd CAS; не используется xdelta-пайплайном
	Strategy     string
	CompressBase bool
	XdeltaPath   string
	DwzPath      string
	ObjcopyPath  string
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
		LazyExtract:      envBool("DEBUGINFOD_LAZY_EXTRACT", true),
		UIEnabled:        envBool("DEBUGINFOD_UI_ENABLED", true),
		CORSOrigins:      splitPaths(envOr("DEBUGINFOD_CORS_ORIGINS", "")),
		RateLimitRPS:     envFloat64("DEBUGINFOD_RATE_LIMIT", 0),
		BasicAuthUser:    envOr("DEBUGINFOD_BASIC_AUTH_USER", ""),
		BasicAuthPass:    envOr("DEBUGINFOD_BASIC_AUTH_PASSWORD", ""),
		TLSCertFile:      envOr("DEBUGINFOD_TLS_CERT", ""),
		TLSKeyFile:       envOr("DEBUGINFOD_TLS_KEY", ""),
		TLSClientCA:      envOr("DEBUGINFOD_TLS_CLIENT_CA", ""),
		MetadataPageSize: envInt("DEBUGINFOD_METADATA_PAGE_SIZE", 100),
		ScanEnabled:      envBool("DEBUGINFOD_SCAN_ENABLED", true),
		AdminKey:         envOr("DEBUGINFOD_ADMIN_KEY", ""),
		ScanWebhookURL:   envOr("DEBUGINFOD_SCAN_WEBHOOK_URL", ""),
		Dedup: DedupConfig{
			Enabled:      envBool("DEBUGINFOD_DEDUP_ENABLED", false),
			Projects:     splitPaths(envOr("DEBUGINFOD_DEDUP_PROJECTS", "")),
			Workers:      envInt("DEBUGINFOD_DEDUP_WORKERS", 4),
			BlobDir:      envOr("DEBUGINFOD_DEDUP_BLOB_DIR", ""),
			Strategy:     envOr("DEBUGINFOD_DEDUP_STRATEGY", "xdelta-decompress-dwz"),
			CompressBase: envBool("DEBUGINFOD_DEDUP_COMPRESS_BASE", true),
			XdeltaPath:   envOr("DEBUGINFOD_XDELTA_PATH", "xdelta3"),
			DwzPath:      envOr("DEBUGINFOD_DWZ_PATH", "dwz"),
			ObjcopyPath:  envOr("DEBUGINFOD_OBJCOPY_PATH", "objcopy"),
		},
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
	flag.BoolVar(&cfg.LazyExtract, "lazy-extract", cfg.LazyExtract, "не кэшировать ELF при индексации, извлекать по HTTP-запросу")
	flag.BoolVar(&cfg.UIEnabled, "ui", cfg.UIEnabled, "включить Web UI на /ui/")
	corsOrigins := strings.Join(cfg.CORSOrigins, ",")
	flag.StringVar(&corsOrigins, "cors-origins", corsOrigins, "CORS origins (через запятую, * = все)")
	flag.Float64Var(&cfg.RateLimitRPS, "rate-limit", cfg.RateLimitRPS, "лимит запросов/с на IP (0=выкл)")
	flag.StringVar(&cfg.BasicAuthUser, "basic-auth-user", cfg.BasicAuthUser, "Basic Auth пользователь")
	flag.StringVar(&cfg.BasicAuthPass, "basic-auth-password", cfg.BasicAuthPass, "Basic Auth пароль")
	flag.StringVar(&cfg.TLSCertFile, "tls-cert", cfg.TLSCertFile, "TLS сертификат")
	flag.StringVar(&cfg.TLSKeyFile, "tls-key", cfg.TLSKeyFile, "TLS ключ")
	flag.StringVar(&cfg.TLSClientCA, "tls-client-ca", cfg.TLSClientCA, "CA для mTLS клиентов")
	flag.IntVar(&cfg.MetadataPageSize, "metadata-page-size", cfg.MetadataPageSize, "размер страницы metadata")
	flag.BoolVar(&cfg.ScanEnabled, "scan-enabled", cfg.ScanEnabled, "включить фоновую индексацию (false = только чтение индекса)")
	adminKey := cfg.AdminKey
	flag.StringVar(&adminKey, "admin-key", adminKey, "токен для /admin/* (по умолчанию DEBUGINFOD_ZABBIX_KEY)")
	scanWebhook := cfg.ScanWebhookURL
	flag.StringVar(&scanWebhook, "scan-webhook-url", scanWebhook, "URL webhook после завершения scan")
	flag.StringVar(&cfg.EnvFile, "env-file", cfg.EnvFile, "путь к .env")
	flag.Parse()

	cfg.AdminKey = adminKey
	cfg.ScanWebhookURL = scanWebhook
	if cfg.AdminKey == "" {
		cfg.AdminKey = cfg.ZabbixKey
	}

	cfg.ScanPaths = splitPaths(scanPath)
	cfg.UpstreamURLs = splitPaths(upstreams)
	cfg.CORSOrigins = splitPaths(corsOrigins)
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

func envBool(key string, fallback bool) bool {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return v
}

func envFloat64(key string, fallback float64) float64 {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return v
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
