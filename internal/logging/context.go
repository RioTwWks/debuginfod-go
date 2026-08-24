package logging

import (
	"context"
	"log/slog"
)

type contextKey struct{}

// WithContext сохраняет request_id в context для вложенных логов.
func WithContext(ctx context.Context, requestID string) context.Context {
	if requestID == "" {
		return ctx
	}
	return context.WithValue(ctx, contextKey{}, requestID)
}

// RequestIDFromContext возвращает request_id из context.
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(contextKey{}).(string)
	return v
}

// LoggerFromContext возвращает логгер с request_id из context.
func LoggerFromContext(ctx context.Context, component string) *slog.Logger {
	logger := WithComponent(component)
	if id := RequestIDFromContext(ctx); id != "" {
		logger = logger.With("request_id", id)
	}
	return logger
}

func contextBackground() context.Context {
	return context.Background()
}
