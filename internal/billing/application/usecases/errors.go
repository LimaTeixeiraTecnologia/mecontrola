package usecases

import "errors"

var ErrFunnelTokenMissing = errors.New("billing: funnel token missing in payload")

var ErrKiwifySubscriptionIDInvalid = errors.New("billing: invalid kiwify subscription id")

var ErrPlanNotFound = errors.New("billing: plan not found for product_id")

var ErrEventAlreadyProcessed = errors.New("billing: event already processed")

var ErrEventSuperseded = errors.New("billing: event superseded by more recent state")

var ErrConcurrentActiveSub = errors.New("billing: user already has an active subscription")

var ErrUnknownTrigger = errors.New("billing: unknown trigger")

var ErrInvalidSignature = errors.New("billing: invalid signature")

var ErrInvalidWebhookPayload = errors.New("billing: invalid webhook payload")
