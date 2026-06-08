package mocks

import mock "github.com/stretchr/testify/mock"

type ExpireTokensUseCase = expireTokensUseCase

func NewExpireTokensUseCase(t interface {
	mock.TestingT
	Cleanup(func())
}) *ExpireTokensUseCase {
	return newExpireTokensUseCase(t)
}
