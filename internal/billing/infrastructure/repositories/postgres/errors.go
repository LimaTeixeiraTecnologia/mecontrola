package postgres

import (
	"errors"

	billinginterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
)

var (
	// ErrSubscriptionNotFound é o alias do sentinel canônico definido em application/interfaces.
	// Garante que os use cases possam usar errors.Is sem depender do pacote infra.
	ErrSubscriptionNotFound        = billinginterfaces.ErrSubscriptionNotFound
	ErrDuplicateActiveSubscription = errors.New("postgres subscription repository: assinatura ativa duplicada para o mesmo usuário")
	ErrWebhookEventNotFound        = errors.New("postgres webhook event repository: evento de webhook não encontrado")
)
