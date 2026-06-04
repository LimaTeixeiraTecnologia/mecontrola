package valueobjects

import "errors"

var (
	ErrUnknownPlanCode             = errors.New("billing: plano desconhecido")
	ErrNegativeAmount              = errors.New("billing: valor monetário não pode ser negativo")
	ErrEmptyPayload                = errors.New("billing: payload não pode ser vazio")
	ErrMalformedPayload            = errors.New("billing: payload json inválido")
	ErrEmptyExternalSubscriptionID = errors.New("billing: external subscription id não pode ser vazio")
	ErrInvalidWebhookEventID       = errors.New("billing: webhook event id inválido")
)
