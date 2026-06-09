package mocks

import mock "github.com/stretchr/testify/mock"

type CreateCheckoutSessionUseCase = createCheckoutSessionUseCase
type GetTokenStateUseCase = getTokenStateUseCase

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
