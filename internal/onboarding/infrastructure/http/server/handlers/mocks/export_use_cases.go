package mocks

import mock "github.com/stretchr/testify/mock"

type ConsumeMagicTokenUseCase = consumeMagicTokenUseCase
type CreateCheckoutSessionUseCase = createCheckoutSessionUseCase
type GetTokenStateUseCase = getTokenStateUseCase
type TryFallbackActivationUseCase = tryFallbackActivationUseCase

func NewConsumeMagicTokenUseCase(t interface {
	mock.TestingT
	Cleanup(func())
}) *ConsumeMagicTokenUseCase {
	return newConsumeMagicTokenUseCase(t)
}

func NewCreateCheckoutSessionUseCase(t interface {
	mock.TestingT
	Cleanup(func())
}) *CreateCheckoutSessionUseCase {
	return newCreateCheckoutSessionUseCase(t)
}

func NewGetTokenStateUseCase(t interface {
	mock.TestingT
	Cleanup(func())
}) *GetTokenStateUseCase {
	return newGetTokenStateUseCase(t)
}

func NewTryFallbackActivationUseCase(t interface {
	mock.TestingT
	Cleanup(func())
}) *TryFallbackActivationUseCase {
	return newTryFallbackActivationUseCase(t)
}
