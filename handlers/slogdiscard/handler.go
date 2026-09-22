package slogdiscard

import (
	"context"
	"log/slog"
)

var _ slog.Handler = (*Handler)(nil)

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) Handle(_ context.Context, _ slog.Record) error {
	// Просто игнорируем запись журнала
	return nil
}

func (h *Handler) WithAttrs(_ []slog.Attr) slog.Handler {
	// Возвращает тот же обработчик, так как нет атрибутов для сохранения
	return h
}

func (h *Handler) WithGroup(_ string) slog.Handler {
	// Возвращает тот же обработчик, так как нет группы для сохранения
	return h
}

func (h *Handler) Enabled(_ context.Context, _ slog.Level) bool {
	// Всегда возвращает false, так как запись журнала игнорируется
	return false
}
