package mocks

import (
	"context"

	mock "github.com/stretchr/testify/mock"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
)

type EntitlementRepository struct {
	mock.Mock
}

type EntitlementRepository_Expecter struct {
	mock *mock.Mock
}

func (_m *EntitlementRepository) EXPECT() *EntitlementRepository_Expecter {
	return &EntitlementRepository_Expecter{mock: &_m.Mock}
}

func (_m *EntitlementRepository) Upsert(ctx context.Context, record interfaces.EntitlementRecord) error {
	ret := _m.Called(ctx, record)
	return ret.Error(0)
}

func (_m *EntitlementRepository) FindByUserID(ctx context.Context, userID string) (interfaces.EntitlementRecord, error) {
	ret := _m.Called(ctx, userID)

	var r0 interfaces.EntitlementRecord
	if rf, ok := ret.Get(0).(func(context.Context, string) interfaces.EntitlementRecord); ok {
		r0 = rf(ctx, userID)
	} else {
		r0 = ret.Get(0).(interfaces.EntitlementRecord)
	}

	return r0, ret.Error(1)
}

func (_m *EntitlementRepository) UpsertPending(ctx context.Context, subscriptionID string, funnelToken string, payload []byte) error {
	ret := _m.Called(ctx, subscriptionID, funnelToken, payload)
	return ret.Error(0)
}

func NewEntitlementRepository(t interface {
	mock.TestingT
	Cleanup(func())
}) *EntitlementRepository {
	m := &EntitlementRepository{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}
