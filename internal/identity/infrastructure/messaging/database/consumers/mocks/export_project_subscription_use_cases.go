package mocks

import mock "github.com/stretchr/testify/mock"

type MockProjectSubscriptionBoundUseCase = projectSubscriptionBoundUseCase
type MockProjectSubscriptionEventUseCase = projectSubscriptionEventUseCase
type MockProjectAuthEventUseCase = projectAuthEventUseCase
type MockAnonymizeUserAuthEventsUseCase = anonymizeUserAuthEventsUseCase

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

func NewMockProjectAuthEventUseCase(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockProjectAuthEventUseCase {
	return newProjectAuthEventUseCase(t)
}

func NewMockAnonymizeUserAuthEventsUseCase(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockAnonymizeUserAuthEventsUseCase {
	return newAnonymizeUserAuthEventsUseCase(t)
}
