package application

import "errors"

var (
	ErrFunnelTokenMissing   = errors.New("onboarding: funnel token missing in billing event")
	ErrCheckoutUnavailable  = errors.New("onboarding: checkout url unavailable for plan")
	ErrUnknownPlan          = errors.New("onboarding: unknown plan id")
	ErrMetaProcessedAlready = errors.New("onboarding: meta message already processed")
	ErrWhatsAppClientError  = errors.New("onboarding: whatsapp client error (4xx) — não reenviar")
	ErrWhatsAppServerError  = errors.New("onboarding: whatsapp server error (5xx) — pode retentar")
)
