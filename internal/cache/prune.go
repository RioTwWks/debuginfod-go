package cache

import (
	"os"
	"path/filepath"
	"sort"
	"time"
)

// DirSize возвращает суммарный размер файлов в каталоге.
func DirSize(dir string) (int64, error) {
	var total int64
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total, err
}

type fileEntry struct {
	path    string
	size    int64
	modTime time.Time
}

// Prune удаляет самые старые файлы, пока размер каталога не станет <= maxBytes.
// maxBytes <= 0 — без ограничений.
func Prune(dir string, maxBytes int64) (removed int, freed int64, err error) {
	if maxBytes <= 0 {
		return 0, 0, nil
	}

	current, err := DirSize(dir)
	if err != nil || current <= maxBytes {
		return 0, 0, err
	}

	var files []fileEntry
	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		files = append(files, fileEntry{path: path, size: info.Size(), modTime: info.ModTime()})
		return nil
	})
	if err != nil {
		return 0, 0, err
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.Before(files[j].modTime)
	})

	for current > maxBytes && len(files) > 0 {
		f := files[0]
		files = files[1:]
		if rmErr := os.Remove(f.path); rmErr != nil {
			continue
		}
		current -= f.size
		freed += f.size
		removed++
	}
	return removed, freed, nil
}
