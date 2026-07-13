package main

import (
    "flag"
    "log"
    "net/http"
    "time"

    "github.com/your-username/debuginfod-go/internal/indexer"
    "github.com/your-username/debuginfod-go/internal/storage"
    "github.com/your-username/debuginfod-go/internal/webapi"
)

func main() {
    // 1. Определяем флаги командной строки
    var (
        dbPath   = flag.String("d", "debuginfod.sqlite", "путь к SQLite базе данных")
        port     = flag.String("p", "8002", "порт для HTTP-сервера")
        scanPath = flag.String("s", ".", "путь для сканирования ELF-файлов")
        rescan   = flag.Duration("r", 1*time.Hour, "интервал переиндексации")
    )
    flag.Parse()

    // 2. Инициализация хранилища (SQLite)
    store, err := storage.New(*dbPath)
    if err != nil {
        log.Fatalf("не удалось открыть БД: %v", err)
    }

    // 3. Запуск индексатора в фоновой горутине
    idx := indexer.NewIndexer(store, []string{*scanPath})
    go func() {
        // Первый проход сразу при старте
        if err := idx.Scan(); err != nil {
            log.Printf("ошибка индексации: %v", err)
        }
        // Затем периодически
        ticker := time.NewTicker(*rescan)
        for range ticker.C {
            if err := idx.Scan(); err != nil {
                log.Printf("ошибка индексации: %v", err)
            }
        }
    }()

    // 4. Настройка HTTP-обработчиков
    handler := webapi.NewHandler(store)
    http.Handle("/buildid/", handler)

    // 5. Запуск сервера
    log.Printf("сервер debuginfod запущен на :%s", *port)
    if err := http.ListenAndServe(":"+*port, nil); err != nil {
        log.Fatalf("сервер упал: %v", err)
    }
}