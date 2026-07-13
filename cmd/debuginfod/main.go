package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/your-username/debuginfod-go/internal/indexer"
	"github.com/your-username/debuginfod-go/internal/storage"
	"github.com/your-username/debuginfod-go/internal/webapi"
)

func main() {
	var (
		dbPath   = flag.String("d", "debuginfod.sqlite", "путь к SQLite базе данных")
		port     = flag.String("p", "8002", "порт для HTTP-сервера")
		scanPath = flag.String("s", ".", "путь для сканирования ELF-файлов")
		rescan   = flag.Duration("r", time.Hour, "интервал переиндексации")
	)
	flag.Parse()

	store, err := storage.New(*dbPath)
	if err != nil {
		log.Fatalf("не удалось открыть БД: %v", err)
	}
	defer store.Close()

	idx := indexer.NewIndexer(store, []string{*scanPath})
	go runIndexer(idx, *rescan)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", webapi.HealthHandler)
	mux.Handle("/buildid/", webapi.NewHandler(store))

	server := &http.Server{
		Addr:              ":" + *port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("сервер debuginfod запущен на :%s (сканирование: %s)", *port, *scanPath)
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
