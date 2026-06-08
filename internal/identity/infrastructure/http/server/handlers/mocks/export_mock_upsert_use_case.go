package mocks

import mock "github.com/stretchr/testify/mock"

type MockUpsertUseCase = upsertUseCase

func NewMockUpsertUseCase(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockUpsertUseCase {
	return newUpsertUseCase(t)
}
