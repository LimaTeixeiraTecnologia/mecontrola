package mocks

import (
	"context"
	"time"

	mock "github.com/stretchr/testify/mock"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type SubscriptionRepository struct {
	mock.Mock
}

type SubscriptionRepository_Expecter struct {
	mock *mock.Mock
}

func (_m *SubscriptionRepository) EXPECT() *SubscriptionRepository_Expecter {
	return &SubscriptionRepository_Expecter{mock: &_m.Mock}
}

func (_m *SubscriptionRepository) FindByOrderID(ctx context.Context, orderID string) (entities.Subscription, error) {
	ret := _m.Called(ctx, orderID)
	if len(ret) == 0 {
		panic("no return value specified for FindByOrderID")
	}
	var r0 entities.Subscription
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (entities.Subscription, error)); ok {
		return rf(ctx, orderID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) entities.Subscription); ok {
		r0 = rf(ctx, orderID)
	} else {
		r0 = ret.Get(0).(entities.Subscription)
	}
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, orderID)
	} else {
		r1 = ret.Error(1)
	}
	return r0, r1
}

func (_m *SubscriptionRepository) FindByUserID(ctx context.Context, userID string) (entities.Subscription, error) {
	ret := _m.Called(ctx, userID)
	if len(ret) == 0 {
		panic("no return value specified for FindByUserID")
	}
	var r0 entities.Subscription
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (entities.Subscription, error)); ok {
		return rf(ctx, userID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) entities.Subscription); ok {
		r0 = rf(ctx, userID)
	} else {
		r0 = ret.Get(0).(entities.Subscription)
	}
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, userID)
	} else {
		r1 = ret.Error(1)
	}
	return r0, r1
}

func (_m *SubscriptionRepository) UpsertByOrder(ctx context.Context, orderID string, sub entities.Subscription, periodStart time.Time) error {
	ret := _m.Called(ctx, orderID, sub, periodStart)
	if len(ret) == 0 {
		panic("no return value specified for UpsertByOrder")
	}
	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, entities.Subscription, time.Time) error); ok {
		r0 = rf(ctx, orderID, sub, periodStart)
	} else {
		r0 = ret.Error(0)
	}
	return r0
}

func (_m *SubscriptionRepository) ExtendPeriod(ctx context.Context, subscriptionID string, newPeriodEnd time.Time, lastEventAt time.Time) error {
	ret := _m.Called(ctx, subscriptionID, newPeriodEnd, lastEventAt)
	if len(ret) == 0 {
		panic("no return value specified for ExtendPeriod")
	}
	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, time.Time, time.Time) error); ok {
		r0 = rf(ctx, subscriptionID, newPeriodEnd, lastEventAt)
	} else {
		r0 = ret.Error(0)
	}
	return r0
}

func (_m *SubscriptionRepository) ApplyTransition(ctx context.Context, subscriptionID string, status valueobjects.Status, graceEnd time.Time, lastEventAt time.Time) error {
	ret := _m.Called(ctx, subscriptionID, status, graceEnd, lastEventAt)
	if len(ret) == 0 {
		panic("no return value specified for ApplyTransition")
	}
	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, valueobjects.Status, time.Time, time.Time) error); ok {
		r0 = rf(ctx, subscriptionID, status, graceEnd, lastEventAt)
	} else {
		r0 = ret.Error(0)
	}
	return r0
}

func (_m *SubscriptionRepository) BindUser(ctx context.Context, subscriptionID string, userID string) error {
	ret := _m.Called(ctx, subscriptionID, userID)
	if len(ret) == 0 {
		panic("no return value specified for BindUser")
	}
	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) error); ok {
		r0 = rf(ctx, subscriptionID, userID)
	} else {
		r0 = ret.Error(0)
	}
	return r0
}

func NewSubscriptionRepository(t interface {
	mock.TestingT
	Cleanup(func())
}) *SubscriptionRepository {
	m := &SubscriptionRepository{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}
