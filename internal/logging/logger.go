package logging

import (
	"context"
	"io"
	"log/slog"
)

var allowed = map[string]bool{
	"request_id": true, "route": true, "method": true, "status": true, "duration_ms": true, "bytes": true,
	"event": true, "outcome": true, "user_id": true, "device_id": true, "container_id": true, "object_id": true,
	"attachment_id": true, "upload_id": true, "session_id": true, "audit_id": true, "count": true,
	"reason_code": true, "retry_after_s": true, "version": true, "error_kind": true,
}

type Logger struct{ *slog.Logger }

func New(w io.Writer, level, format string) *Logger {
	var h slog.Handler
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if level == "debug" {
		opts.Level = slog.LevelDebug
	}
	if level == "warn" {
		opts.Level = slog.LevelWarn
	}
	if level == "error" {
		opts.Level = slog.LevelError
	}
	if format == "text" {
		h = slog.NewTextHandler(w, opts)
	} else {
		h = slog.NewJSONHandler(w, opts)
	}
	return &Logger{slog.New(&filterHandler{Handler: h})}
}

type filterHandler struct{ slog.Handler }

func (h *filterHandler) Handle(ctx context.Context, r slog.Record) error {
	var dropped int
	attrs := []slog.Attr{}
	r.Attrs(func(a slog.Attr) bool {
		if allowed[a.Key] {
			attrs = append(attrs, a)
		} else {
			dropped++
		}
		return true
	})
	if dropped > 0 {
		attrs = append(attrs, slog.Int("dropped_fields", dropped))
	}
	nr := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	nr.AddAttrs(attrs...)
	return h.Handler.Handle(ctx, nr)
}
func (h *filterHandler) WithAttrs(a []slog.Attr) slog.Handler {
	filtered := make([]slog.Attr, 0, len(a))
	for _, x := range a {
		if allowed[x.Key] {
			filtered = append(filtered, x)
		}
	}
	return &filterHandler{Handler: h.Handler.WithAttrs(filtered)}
}
func (h *filterHandler) WithGroup(s string) slog.Handler { return h.Handler.WithGroup(s) }
