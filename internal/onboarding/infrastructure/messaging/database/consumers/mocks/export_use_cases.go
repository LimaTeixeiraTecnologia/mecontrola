package mocks

import mock "github.com/stretchr/testify/mock"

type HandlePaidWithoutTokenUseCase = handlePaidWithoutTokenUseCase
type MarkTokenPaidUseCase = markTokenPaidUseCase

func NewHandlePaidWithoutTokenUseCase(t interface {
	mock.TestingT
	Cleanup(func())
}) *HandlePaidWithoutTokenUseCase {
	return newHandlePaidWithoutTokenUseCase(t)
}

func NewMarkTokenPaidUseCase(t interface {
	mock.TestingT
	Cleanup(func())
}) *MarkTokenPaidUseCase {
	return newMarkTokenPaidUseCase(t)
}
