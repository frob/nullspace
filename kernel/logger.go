package kernel

import "log/slog"

// slogLogger wraps log/slog to implement the Logger port.
type slogLogger struct {
	inner *slog.Logger
}

// NewSlogLogger creates a Logger backed by log/slog with default settings.
func NewSlogLogger() Logger {
	return &slogLogger{inner: slog.Default()}
}

// NewSlogLoggerFrom creates a Logger backed by the given slog.Logger.
func NewSlogLoggerFrom(l *slog.Logger) Logger {
	return &slogLogger{inner: l}
}

func (l *slogLogger) Debug(msg string, args ...any) { l.inner.Debug(msg, args...) }
func (l *slogLogger) Info(msg string, args ...any)  { l.inner.Info(msg, args...) }
func (l *slogLogger) Warn(msg string, args ...any)  { l.inner.Warn(msg, args...) }
func (l *slogLogger) Error(msg string, args ...any) { l.inner.Error(msg, args...) }

func (l *slogLogger) With(args ...any) Logger {
	return &slogLogger{inner: l.inner.With(args...)}
}
