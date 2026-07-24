package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/your-username/debuginfod-go/internal/cache"
	"github.com/your-username/debuginfod-go/internal/config"
	"github.com/your-username/debuginfod-go/internal/dedup"
	"github.com/your-username/debuginfod-go/internal/federation"
	"github.com/your-username/debuginfod-go/internal/indexer"
	"github.com/your-username/debuginfod-go/internal/logging"
	"github.com/your-username/debuginfod-go/internal/metrics"
	"github.com/your-username/debuginfod-go/internal/scanrunner"
	"github.com/your-username/debuginfod-go/internal/storage"
	"github.com/your-username/debuginfod-go/internal/webapi"
)

func main() {
	cfg := config.Load()
	_, logCloser := logging.Setup(logging.Options{
		Level:  cfg.LogLevel,
		LogDir: cfg.LogDir,
	})
	if logCloser != nil {
		defer func() { _ = logCloser.Close() }()
	}

	if err := os.MkdirAll(cfg.CacheDir, 0o755); err != nil {
		slog.Error("create cache dir", "err", err)
		os.Exit(1)
	}
	if cfg.Dedup.Enabled {
		if err := os.MkdirAll(dedupBlobDir(cfg), 0o755); err != nil {
			slog.Error("create dedup blob dir", "err", err)
			os.Exit(1)
		}
	}

	store, err := storage.Open(cfg.DBPath, cfg.DatabaseURL)
	if err != nil {
		slog.Error("open database", "err", err)
		os.Exit(1)
	}
	defer store.Close()

	collector := metrics.New()
	fed := federation.New(cfg.UpstreamURLs, 30*time.Second)

	idx := indexer.NewIndexer(indexer.Options{
		Storage:       store,
		Paths:         cfg.ScanPaths,
		CacheDir:      cfg.CacheDir,
		CacheMaxBytes: cfg.CacheMaxBytes,
		Workers:       cfg.ScanWorkers,
		Metrics:       collector,
		LazyExtract:   cfg.LazyExtract,
	})

	dedupSvc := dedup.NewService(store, cfg.Dedup, cfg.ScanPaths, dedupBlobDir(cfg))
	dedupAdapter := webapi.NewDedupAdapter(dedupSvc)

	runner := scanrunner.New(scanrunner.Options{
		Indexer:    idx,
		Metrics:    collector,
		Interval:   cfg.RescanInterval,
		Enabled:    cfg.ScanEnabled,
		WebhookURL: cfg.ScanWebhookURL,
		Dedup:      dedup.NewScanHook(dedupSvc, collector),
	})

	scanCtx, cancelScan := context.WithCancel(context.Background())
	defer cancelScan()
	go runner.Run(scanCtx)

	rescanSig := make(chan os.Signal, 1)
	signal.Notify(rescanSig, syscall.SIGUSR1)
	go func() {
		for range rescanSig {
			slog.Info("SIGUSR1 received, triggering rescan")
			runner.Trigger()
		}
	}()

	cacheBytes := func() int64 {
		n, _ := cache.DirSize(cfg.CacheDir)
		return n
	}

	security := webapi.SecurityOpts{
		CORSOrigins:   cfg.CORSOrigins,
		RateLimitRPS:  cfg.RateLimitRPS,
		BasicAuthUser: cfg.BasicAuthUser,
		BasicAuthPass: cfg.BasicAuthPass,
		TLSCertFile:   cfg.TLSCertFile,
		TLSKeyFile:    cfg.TLSKeyFile,
		TLSClientCA:   cfg.TLSClientCA,
	}

	var scanTrigger webapi.ScanTrigger
	if cfg.ScanEnabled {
		scanTrigger = runner
	}

	opts := webapi.ServerOpts{
		Store:            store,
		MetadataMaxTime:  cfg.MetadataMaxTime,
		MetadataPageSize: cfg.MetadataPageSize,
		Federation:       fed,
		Metrics:          collector,
		ZabbixKey:        cfg.ZabbixKey,
		AdminKey:         cfg.AdminKey,
		ScanTrigger:      scanTrigger,
		DedupRunner:      dedupAdapter,
		DedupRestorer:    dedupAdapter,
		CacheBytes:       cacheBytes,
		CacheDir:         cfg.CacheDir,
		ScanPaths:        cfg.ScanPaths,
		UIEnabled:        cfg.UIEnabled,
		DedupEnabled:     cfg.Dedup.Enabled,
		Security:         security,
	}

	tlsConfig, err := security.BuildTLSConfig()
	if err != nil {
		slog.Error("tls config", "err", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           webapi.NewMux(opts),
		ReadHeaderTimeout: 10 * time.Second,
		TLSConfig:         tlsConfig,
	}

	go func() {
		slog.Info("debuginfod started",
			"port", cfg.Port,
			"tls", security.TLSConfigured(),
			"mtls", security.TLSClientCA != "",
			"basic_auth", security.BasicAuthEnabled(),
			"cors", len(cfg.CORSOrigins) > 0,
			"rate_limit", cfg.RateLimitRPS,
			"scan_paths", cfg.ScanPaths,
			"scan_enabled", cfg.ScanEnabled,
			"dedup_enabled", cfg.Dedup.Enabled,
			"workers", cfg.ScanWorkers,
			"federation", len(cfg.UpstreamURLs) > 0,
			"scan_webhook", cfg.ScanWebhookURL != "",
		)
		var serveErr error
		if security.TLSConfigured() {
			serveErr = server.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile)
		} else {
			serveErr = server.ListenAndServe()
		}
		if serveErr != nil && serveErr != http.ErrServerClosed {
			slog.Error("server failed", "err", serveErr)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	cancelScan()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("shutdown", "err", err)
	}
	slog.Info("server stopped")
}

func dedupBlobDir(cfg config.Config) string {
	if cfg.Dedup.BlobDir != "" {
		return cfg.Dedup.BlobDir
	}
	return filepath.Join(cfg.CacheDir, "dedup-blobs")
}
