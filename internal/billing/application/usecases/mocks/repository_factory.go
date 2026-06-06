package mocks

import (
	"github.com/JailtonJunior94/devkit-go/pkg/database"
	mock "github.com/stretchr/testify/mock"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
)

type RepositoryFactory struct {
	mock.Mock
}

type RepositoryFactory_Expecter struct {
	mock *mock.Mock
}

func (_m *RepositoryFactory) EXPECT() *RepositoryFactory_Expecter {
	return &RepositoryFactory_Expecter{mock: &_m.Mock}
}

func (_m *RepositoryFactory) SubscriptionRepository(db database.DBTX) interfaces.SubscriptionRepository {
	ret := _m.Called(db)
	if len(ret) == 0 {
		panic("no return value specified for SubscriptionRepository")
	}
	var r0 interfaces.SubscriptionRepository
	if rf, ok := ret.Get(0).(func(database.DBTX) interfaces.SubscriptionRepository); ok {
		r0 = rf(db)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(interfaces.SubscriptionRepository)
		}
	}
	return r0
}

func (_m *RepositoryFactory) ProcessedEventRepository(db database.DBTX) interfaces.ProcessedEventRepository {
	ret := _m.Called(db)
	if len(ret) == 0 {
		panic("no return value specified for ProcessedEventRepository")
	}
	var r0 interfaces.ProcessedEventRepository
	if rf, ok := ret.Get(0).(func(database.DBTX) interfaces.ProcessedEventRepository); ok {
		r0 = rf(db)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(interfaces.ProcessedEventRepository)
		}
	}
	return r0
}

func (_m *RepositoryFactory) KiwifyEventRepository(db database.DBTX) interfaces.KiwifyEventRepository {
	ret := _m.Called(db)
	if len(ret) == 0 {
		panic("no return value specified for KiwifyEventRepository")
	}
	var r0 interfaces.KiwifyEventRepository
	if rf, ok := ret.Get(0).(func(database.DBTX) interfaces.KiwifyEventRepository); ok {
		r0 = rf(db)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(interfaces.KiwifyEventRepository)
		}
	}
	return r0
}

func (_m *RepositoryFactory) PlanRepository(db database.DBTX) interfaces.PlanRepository {
	ret := _m.Called(db)
	if len(ret) == 0 {
		panic("no return value specified for PlanRepository")
	}
	var r0 interfaces.PlanRepository
	if rf, ok := ret.Get(0).(func(database.DBTX) interfaces.PlanRepository); ok {
		r0 = rf(db)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(interfaces.PlanRepository)
		}
	}
	return r0
}

func (_m *RepositoryFactory) ReconciliationCheckpointRepository(db database.DBTX) interfaces.ReconciliationCheckpointRepository {
	ret := _m.Called(db)
	if len(ret) == 0 {
		panic("no return value specified for ReconciliationCheckpointRepository")
	}
	var r0 interfaces.ReconciliationCheckpointRepository
	if rf, ok := ret.Get(0).(func(database.DBTX) interfaces.ReconciliationCheckpointRepository); ok {
		r0 = rf(db)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(interfaces.ReconciliationCheckpointRepository)
		}
	}
	return r0
}

func NewRepositoryFactory(t interface {
	mock.TestingT
	Cleanup(func())
}) *RepositoryFactory {
	m := &RepositoryFactory{}
	m.Test(t)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return m
}
