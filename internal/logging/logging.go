package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// Options — настройки slog.
type Options struct {
	// Level — уровень для stdout (debug|info|warn|error).
	Level string
	// LogDir — каталог ежедневных файлов; пусто — только stdout.
	LogDir string
	// FilePrefix — префикс имени файла (debuginfod-YYYY-MM-DD.log).
	FilePrefix string
}

// Setup настраивает глобальный slog-логгер.
// Возвращает logger и closer для файла (может быть nil).
func Setup(opts Options) (*slog.Logger, io.Closer) {
	consoleLevel := parseLevel(opts.Level)
	consoleHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: consoleLevel})

	var handlers []slog.Handler
	handlers = append(handlers, consoleHandler)

	var closer io.Closer
	if dir := strings.TrimSpace(opts.LogDir); dir != "" {
		prefix := strings.TrimSpace(opts.FilePrefix)
		if prefix == "" {
			prefix = "debuginfod"
		}
		writer, err := newDailyWriter(dir, prefix)
		if err != nil {
			// fallback: только stdout, ошибку залогируем после SetDefault
			logger := slog.New(consoleHandler)
			slog.SetDefault(logger)
			slog.Error("file logging disabled", "dir", dir, "err", err)
			return logger, nil
		}
		closer = writer
		fileHandler := slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: slog.LevelDebug})
		handlers = append(handlers, fileHandler)
	}

	logger := slog.New(newMultiHandler(handlers...))
	slog.SetDefault(logger)
	return logger, closer
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
