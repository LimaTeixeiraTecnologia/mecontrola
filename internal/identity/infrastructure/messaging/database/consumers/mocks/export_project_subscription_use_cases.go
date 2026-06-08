package mocks

import mock "github.com/stretchr/testify/mock"

type MockProjectSubscriptionBoundUseCase = projectSubscriptionBoundUseCase
type MockProjectSubscriptionEventUseCase = projectSubscriptionEventUseCase

func NewMockProjectSubscriptionBoundUseCase(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockProjectSubscriptionBoundUseCase {
	return newProjectSubscriptionBoundUseCase(t)
}

func NewMockProjectSubscriptionEventUseCase(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockProjectSubscriptionEventUseCase {
	return newProjectSubscriptionEventUseCase(t)
}
