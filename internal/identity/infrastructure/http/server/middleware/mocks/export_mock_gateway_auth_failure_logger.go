package mocks

import mock "github.com/stretchr/testify/mock"

type MockGatewayAuthFailureLogger = gatewayAuthFailureLogger

func NewMockGatewayAuthFailureLogger(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockGatewayAuthFailureLogger {
	return newGatewayAuthFailureLogger(t)
}
