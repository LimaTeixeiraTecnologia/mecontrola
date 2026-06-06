package mocks

import (
	"context"
	"time"

	mock "github.com/stretchr/testify/mock"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
)

type KiwifyClient struct {
	mock.Mock
}

type KiwifyClient_Expecter struct {
	mock *mock.Mock
}

func (_m *KiwifyClient) EXPECT() *KiwifyClient_Expecter {
	return &KiwifyClient_Expecter{mock: &_m.Mock}
}

func (_m *KiwifyClient) ListSalesUpdatedSince(ctx context.Context, windowStart time.Time, windowEnd time.Time, page int) (interfaces.KiwifySalePage, error) {
	ret := _m.Called(ctx, windowStart, windowEnd, page)
	if len(ret) == 0 {
		panic("no return value specified for ListSalesUpdatedSince")
	}
	var r0 interfaces.KiwifySalePage
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, time.Time, time.Time, int) (interfaces.KiwifySalePage, error)); ok {
		return rf(ctx, windowStart, windowEnd, page)
	}
	if rf, ok := ret.Get(0).(func(context.Context, time.Time, time.Time, int) interfaces.KiwifySalePage); ok {
		r0 = rf(ctx, windowStart, windowEnd, page)
	} else {
		r0 = ret.Get(0).(interfaces.KiwifySalePage)
	}
	if rf, ok := ret.Get(1).(func(context.Context, time.Time, time.Time, int) error); ok {
		r1 = rf(ctx, windowStart, windowEnd, page)
	} else {
		r1 = ret.Error(1)
	}
	return r0, r1
}

func (_m *KiwifyClient) GetSale(ctx context.Context, saleID string) (interfaces.KiwifySale, error) {
	ret := _m.Called(ctx, saleID)
	if len(ret) == 0 {
		panic("no return value specified for GetSale")
	}
	var r0 interfaces.KiwifySale
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (interfaces.KiwifySale, error)); ok {
		return rf(ctx, saleID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) interfaces.KiwifySale); ok {
		r0 = rf(ctx, saleID)
	} else {
		r0 = ret.Get(0).(interfaces.KiwifySale)
	}
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, saleID)
	} else {
		r1 = ret.Error(1)
	}
	return r0, r1
}

func NewKiwifyClient(t interface {
	mock.TestingT
	Cleanup(func())
}) *KiwifyClient {
	m := &KiwifyClient{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}
