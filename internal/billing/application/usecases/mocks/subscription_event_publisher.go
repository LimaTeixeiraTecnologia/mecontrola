package mocks

import (
	"context"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	mock "github.com/stretchr/testify/mock"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
)

type SubscriptionEventPublisher struct {
	mock.Mock
}

type SubscriptionEventPublisher_Expecter struct {
	mock *mock.Mock
}

func (_m *SubscriptionEventPublisher) EXPECT() *SubscriptionEventPublisher_Expecter {
	return &SubscriptionEventPublisher_Expecter{mock: &_m.Mock}
}

func (_m *SubscriptionEventPublisher) PublishActivated(ctx context.Context, tx database.DBTX, sub entities.Subscription, subscriptionID string, funnelToken string) error {
	ret := _m.Called(ctx, tx, sub, subscriptionID, funnelToken)
	if len(ret) == 0 {
		panic("no return value specified for PublishActivated")
	}
	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, database.DBTX, entities.Subscription, string, string) error); ok {
		r0 = rf(ctx, tx, sub, subscriptionID, funnelToken)
	} else {
		r0 = ret.Error(0)
	}
	return r0
}

func (_m *SubscriptionEventPublisher) PublishRenewed(ctx context.Context, tx database.DBTX, sub entities.Subscription, subscriptionID string, previousPeriodEnd time.Time) error {
	ret := _m.Called(ctx, tx, sub, subscriptionID, previousPeriodEnd)
	if len(ret) == 0 {
		panic("no return value specified for PublishRenewed")
	}
	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, database.DBTX, entities.Subscription, string, time.Time) error); ok {
		r0 = rf(ctx, tx, sub, subscriptionID, previousPeriodEnd)
	} else {
		r0 = ret.Error(0)
	}
	return r0
}

func (_m *SubscriptionEventPublisher) PublishPastDue(ctx context.Context, tx database.DBTX, sub entities.Subscription, subscriptionID string) error {
	ret := _m.Called(ctx, tx, sub, subscriptionID)
	if len(ret) == 0 {
		panic("no return value specified for PublishPastDue")
	}
	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, database.DBTX, entities.Subscription, string) error); ok {
		r0 = rf(ctx, tx, sub, subscriptionID)
	} else {
		r0 = ret.Error(0)
	}
	return r0
}

func (_m *SubscriptionEventPublisher) PublishCanceled(ctx context.Context, tx database.DBTX, sub entities.Subscription, subscriptionID string) error {
	ret := _m.Called(ctx, tx, sub, subscriptionID)
	if len(ret) == 0 {
		panic("no return value specified for PublishCanceled")
	}
	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, database.DBTX, entities.Subscription, string) error); ok {
		r0 = rf(ctx, tx, sub, subscriptionID)
	} else {
		r0 = ret.Error(0)
	}
	return r0
}

func (_m *SubscriptionEventPublisher) PublishRefunded(ctx context.Context, tx database.DBTX, sub entities.Subscription, subscriptionID string) error {
	ret := _m.Called(ctx, tx, sub, subscriptionID)
	if len(ret) == 0 {
		panic("no return value specified for PublishRefunded")
	}
	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, database.DBTX, entities.Subscription, string) error); ok {
		r0 = rf(ctx, tx, sub, subscriptionID)
	} else {
		r0 = ret.Error(0)
	}
	return r0
}

func NewSubscriptionEventPublisher(t interface {
	mock.TestingT
	Cleanup(func())
}) *SubscriptionEventPublisher {
	m := &SubscriptionEventPublisher{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}
