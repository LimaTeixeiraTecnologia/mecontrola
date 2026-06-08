package mocks

import mock "github.com/stretchr/testify/mock"

type MockEntitlementRepository = EntitlementRepository
type MockRepositoryFactory = RepositoryFactory
type MockUserRepository = UserRepository

func NewMockEntitlementRepository(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockEntitlementRepository {
	return NewEntitlementRepository(t)
}

func NewMockRepositoryFactory(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockRepositoryFactory {
	return NewRepositoryFactory(t)
}

func NewMockUserRepository(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockUserRepository {
	return NewUserRepository(t)
}
