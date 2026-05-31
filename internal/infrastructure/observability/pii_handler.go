package observability

import (
	"context"
	"log/slog"
	"strings"
)

const redacted = "[REDACTED]"

// piiHandler é um slog.Handler wrapper que mascara os valores dos atributos cujos
// nomes estejam em PIIFields antes de delegar ao handler downstream.
//
// Uso obrigatório: wrappear qualquer handler slog que emita telemetria de produção,
// garantindo que phone, amount e demais campos da lista não apareçam em claro.
type piiHandler struct {
	inner slog.Handler
}

// NewRedactingSlogHandler retorna um slog.Handler que intercepta todos os atributos,
// substitui por "[REDACTED]" aqueles cujo nome (case-insensitive) conste em PIIFields,
// e delega ao handler downstream para a escrita real.
//
// Invariante: nenhum campo de PIIFields pode sair em claro para o handler inner.
func NewRedactingSlogHandler(inner slog.Handler) slog.Handler {
	return &piiHandler{inner: inner}
}

func (h *piiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *piiHandler) Handle(ctx context.Context, r slog.Record) error {
	sanitized := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		sanitized.AddAttrs(redactAttr(a))
		return true
	})
	return h.inner.Handle(ctx, sanitized)
}

func (h *piiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	redactedAttrs := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		redactedAttrs[i] = redactAttr(a)
	}
	return &piiHandler{inner: h.inner.WithAttrs(redactedAttrs)}
}

func (h *piiHandler) WithGroup(name string) slog.Handler {
	return &piiHandler{inner: h.inner.WithGroup(name)}
}

// redactAttr substitui o valor de um atributo cujo nome seja PII por "[REDACTED]".
// Grupos são processados recursivamente.
func redactAttr(a slog.Attr) slog.Attr {
	// Resolve o valor (pode ser um LogValuer)
	a.Value = a.Value.Resolve()

	if a.Value.Kind() == slog.KindGroup {
		group := a.Value.Group()
		redactedGroup := make([]slog.Attr, len(group))
		for i, child := range group {
			redactedGroup[i] = redactAttr(child)
		}
		return slog.Group(a.Key, attrsToAny(redactedGroup)...)
	}

	if isPIIKey(a.Key) {
		return slog.String(a.Key, redacted)
	}
	return a
}

// isPIIKey verifica se key (case-insensitive) corresponde a algum campo de PIIFields.
func isPIIKey(key string) bool {
	lower := strings.ToLower(key)
	for _, field := range PIIFields {
		if strings.ToLower(field) == lower {
			return true
		}
	}
	return false
}

// attrsToAny converte []slog.Attr para []any para uso com slog.Group.
func attrsToAny(attrs []slog.Attr) []any {
	result := make([]any, len(attrs))
	for i, a := range attrs {
		result[i] = a
	}
	return result
}
