package indexer

import (
    "os"
    "path/filepath"
    "debuginfod-go/pkg/buildid"
    "debuginfod-go/internal/storage"
    "log"
)

type Indexer struct {
    storage *storage.Storage
    paths   []string
}

func NewIndexer(storage *storage.Storage, paths []string) *Indexer {
    return &Indexer{storage: storage, paths: paths}
}

func (i *Indexer) Scan() error {
    for _, root := range i.paths {
        err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
            if err != nil {
                return nil // Пропускаем ошибки доступа
            }
            if info.IsDir() {
                return nil
            }
            // Пытаемся извлечь build-id
            id, err := buildid.FromPath(path)
            if err != nil {
                return nil // Не ELF или нет build-id
            }
            // Определяем тип (упрощенно: по расширению или имени)
            artifactType := "executable" // или "debuginfo"
            if filepath.Base(path) == "foo.debug" {
                artifactType = "debuginfo"
            }
            // Сохраняем в базу
            if err := i.storage.AddArtifact(id, path, artifactType); err != nil {
                log.Printf("Error saving %s: %v", path, err)
            }
            return nil
        })
        if err != nil {
            log.Printf("Walk error: %v", err)
        }
    }
    return nil
}
