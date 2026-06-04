package interfaces

import (
	"context"
	"errors"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/services"
)

var ErrUnknownProviderEventType = errors.New("billing provider: tipo de evento desconhecido")

// BillingProvider é o port hexagonal para o provedor de cobrança externo (Kiwify no MVP).
// O secret de verificação é injetado no adapter via construtor — não aparece na assinatura
// para manter a interface estável entre provedores (ADR-006).
type BillingProvider interface {
	// VerifySignature verifica a autenticidade do payload recebido a partir dos headers.
	// Retorna erro em assinatura inválida ou ausente.
	VerifySignature(payload []byte, headers map[string]string) error
	ParseEvent(payload []byte) (services.CanonicalEvent, error)
	FetchSubscription(ctx context.Context, externalSubscriptionID string) (services.CanonicalSubscription, error)
}
