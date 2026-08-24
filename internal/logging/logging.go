package logging

import (
	"fmt"
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
	consoleHandler := slog.NewJSONHandler(os.Stdout, handlerOptions(consoleLevel))

	var handlers []slog.Handler
	handlers = append(handlers, consoleHandler)

	var closer io.Closer
	logDir := strings.TrimSpace(opts.LogDir)
	if logDir != "" {
		prefix := strings.TrimSpace(opts.FilePrefix)
		if prefix == "" {
			prefix = "debuginfod"
		}
		writer, err := newDailyWriter(logDir, prefix)
		if err != nil {
			logger := slog.New(consoleHandler)
			slog.SetDefault(logger)
			slog.Error("file logging disabled", "dir", logDir, "err", err)
			return logger, nil
		}
		closer = writer
		fileHandler := slog.NewJSONHandler(writer, handlerOptions(slog.LevelDebug))
		handlers = append(handlers, fileHandler)
	}

	logger := slog.New(newMultiHandler(handlers...))
	slog.SetDefault(logger)
	slog.Info("logging configured",
		"console_level", opts.Level,
		"log_dir", logDir,
		"file_level", fileLogLevel(logDir),
	)
	return logger, closer
}

// WithComponent возвращает логгер с полем component.
func WithComponent(name string) *slog.Logger {
	return slog.Default().With("component", name)
}

// DebugEnabled возвращает true, если глобальный логгер пишет DEBUG.
func DebugEnabled() bool {
	return slog.Default().Enabled(contextBackground(), slog.LevelDebug)
}

func handlerOptions(level slog.Level) *slog.HandlerOptions {
	return &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.SourceKey {
				if src, ok := a.Value.Any().(*slog.Source); ok && src != nil {
					a.Value = slog.StringValue(shortSource(src.File, src.Line))
				}
			}
			return a
		},
	}
}

func shortSource(file string, line int) string {
	if idx := strings.LastIndex(file, "/internal/"); idx >= 0 {
		file = file[idx+len("/internal/"):]
	} else if idx := strings.LastIndex(file, "/pkg/"); idx >= 0 {
		file = file[idx+len("/pkg/"):]
	} else if idx := strings.LastIndex(file, "/cmd/"); idx >= 0 {
		file = file[idx+len("/cmd/"):]
	}
	return fmt.Sprintf("%s:%d", file, line)
}

func fileLogLevel(logDir string) string {
	if logDir == "" {
		return "disabled"
	}
	return "debug"
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
