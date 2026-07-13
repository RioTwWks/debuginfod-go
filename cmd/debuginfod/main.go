package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/your-username/debuginfod-go/internal/config"
	"github.com/your-username/debuginfod-go/internal/indexer"
	"github.com/your-username/debuginfod-go/internal/storage"
	"github.com/your-username/debuginfod-go/internal/webapi"
)

func main() {
	cfg := config.Load()

	if err := os.MkdirAll(cfg.CacheDir, 0o755); err != nil {
		log.Fatalf("не удалось создать cache dir: %v", err)
	}

	store, err := storage.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("не удалось открыть БД: %v", err)
	}
	defer store.Close()

	idx := indexer.NewIndexer(store, cfg.ScanPaths, cfg.CacheDir)
	go runIndexer(idx, cfg.RescanInterval)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", webapi.HealthHandler)
	mux.HandleFunc("/metadata", webapi.MetadataHandler(store))
	mux.Handle("/buildid/", webapi.NewHandler(store))

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("сервер debuginfod запущен на :%s (сканирование: %v)", cfg.Port, cfg.ScanPaths)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("сервер упал: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("ошибка остановки сервера: %v", err)
	}
	log.Println("сервер остановлен")
}

func runIndexer(idx *indexer.Indexer, interval time.Duration) {
	if err := idx.Scan(); err != nil {
		log.Printf("ошибка индексации: %v", err)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		if err := idx.Scan(); err != nil {
			log.Printf("ошибка индексации: %v", err)
		}
	}
}
