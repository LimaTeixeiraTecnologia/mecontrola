package mocks

import (
	"context"

	mock "github.com/stretchr/testify/mock"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type PlanRepository struct {
	mock.Mock
}

type PlanRepository_Expecter struct {
	mock *mock.Mock
}

func (_m *PlanRepository) EXPECT() *PlanRepository_Expecter {
	return &PlanRepository_Expecter{mock: &_m.Mock}
}

func (_m *PlanRepository) FindByKiwifyProductID(ctx context.Context, kiwifyProductID string) (valueobjects.Plan, error) {
	ret := _m.Called(ctx, kiwifyProductID)
	if len(ret) == 0 {
		panic("no return value specified for FindByKiwifyProductID")
	}
	var r0 valueobjects.Plan
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (valueobjects.Plan, error)); ok {
		return rf(ctx, kiwifyProductID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) valueobjects.Plan); ok {
		r0 = rf(ctx, kiwifyProductID)
	} else {
		r0 = ret.Get(0).(valueobjects.Plan)
	}
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, kiwifyProductID)
	} else {
		r1 = ret.Error(1)
	}
	return r0, r1
}

func (_m *PlanRepository) FindByCode(ctx context.Context, code valueobjects.PlanCode) (valueobjects.Plan, error) {
	ret := _m.Called(ctx, code)
	if len(ret) == 0 {
		panic("no return value specified for FindByCode")
	}
	var r0 valueobjects.Plan
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, valueobjects.PlanCode) (valueobjects.Plan, error)); ok {
		return rf(ctx, code)
	}
	if rf, ok := ret.Get(0).(func(context.Context, valueobjects.PlanCode) valueobjects.Plan); ok {
		r0 = rf(ctx, code)
	} else {
		r0 = ret.Get(0).(valueobjects.Plan)
	}
	if rf, ok := ret.Get(1).(func(context.Context, valueobjects.PlanCode) error); ok {
		r1 = rf(ctx, code)
	} else {
		r1 = ret.Error(1)
	}
	return r0, r1
}

func (_m *PlanRepository) ConfigureProductIDs(ctx context.Context, productIDs map[valueobjects.PlanCode]string) error {
	ret := _m.Called(ctx, productIDs)
	if len(ret) == 0 {
		panic("no return value specified for ConfigureProductIDs")
	}
	return ret.Error(0)
}

func NewPlanRepository(t interface {
	mock.TestingT
	Cleanup(func())
}) *PlanRepository {
	m := &PlanRepository{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}
