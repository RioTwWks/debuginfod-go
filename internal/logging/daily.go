package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// dailyWriter пишет в файл с именем по текущей дате (UTC).
type dailyWriter struct {
	dir    string
	prefix string
	mu     sync.Mutex
	day    string
	file   *os.File
}

func newDailyWriter(dir, prefix string) (*dailyWriter, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}
	w := &dailyWriter{dir: dir, prefix: prefix}
	if err := w.rotateIfNeeded(time.Now().UTC()); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *dailyWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.rotateIfNeeded(time.Now().UTC()); err != nil {
		return 0, err
	}
	return w.file.Write(p)
}

func (w *dailyWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	w.day = ""
	return err
}

func (w *dailyWriter) rotateIfNeeded(now time.Time) error {
	day := now.Format("2006-01-02")
	if w.file != nil && w.day == day {
		return nil
	}
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
	}
	name := fmt.Sprintf("%s-%s.log", w.prefix, day)
	path := filepath.Join(w.dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open log file %s: %w", path, err)
	}
	w.file = f
	w.day = day
	return nil
}
