package mocks

import (
	mock "github.com/stretchr/testify/mock"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
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

func (_m *RepositoryFactory) AuthEventsRepository(db database.DBTX) interfaces.AuthEventsRepository {
	ret := _m.Called(db)

	if len(ret) == 0 {
		panic("no return value specified for AuthEventsRepository")
	}

	var r0 interfaces.AuthEventsRepository
	if rf, ok := ret.Get(0).(func(database.DBTX) interfaces.AuthEventsRepository); ok {
		r0 = rf(db)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(interfaces.AuthEventsRepository)
		}
	}

	return r0
}

type RepositoryFactory_AuthEventsRepository_Call struct {
	*mock.Call
}

func (_e *RepositoryFactory_Expecter) AuthEventsRepository(db any) *RepositoryFactory_AuthEventsRepository_Call {
	return &RepositoryFactory_AuthEventsRepository_Call{Call: _e.mock.On("AuthEventsRepository", db)}
}

func (_c *RepositoryFactory_AuthEventsRepository_Call) Run(run func(db database.DBTX)) *RepositoryFactory_AuthEventsRepository_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(database.DBTX))
	})
	return _c
}

func (_c *RepositoryFactory_AuthEventsRepository_Call) Return(_a0 interfaces.AuthEventsRepository) *RepositoryFactory_AuthEventsRepository_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *RepositoryFactory_AuthEventsRepository_Call) RunAndReturn(run func(database.DBTX) interfaces.AuthEventsRepository) *RepositoryFactory_AuthEventsRepository_Call {
	_c.Call.Return(run)
	return _c
}

func (_m *RepositoryFactory) EntitlementRepository(db database.DBTX) interfaces.EntitlementRepository {
	ret := _m.Called(db)

	if len(ret) == 0 {
		panic("no return value specified for EntitlementRepository")
	}

	var r0 interfaces.EntitlementRepository
	if rf, ok := ret.Get(0).(func(database.DBTX) interfaces.EntitlementRepository); ok {
		r0 = rf(db)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(interfaces.EntitlementRepository)
		}
	}

	return r0
}

type RepositoryFactory_EntitlementRepository_Call struct {
	*mock.Call
}

func (_e *RepositoryFactory_Expecter) EntitlementRepository(db any) *RepositoryFactory_EntitlementRepository_Call {
	return &RepositoryFactory_EntitlementRepository_Call{Call: _e.mock.On("EntitlementRepository", db)}
}

func (_c *RepositoryFactory_EntitlementRepository_Call) Run(run func(db database.DBTX)) *RepositoryFactory_EntitlementRepository_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(database.DBTX))
	})
	return _c
}

func (_c *RepositoryFactory_EntitlementRepository_Call) Return(_a0 interfaces.EntitlementRepository) *RepositoryFactory_EntitlementRepository_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *RepositoryFactory_EntitlementRepository_Call) RunAndReturn(run func(database.DBTX) interfaces.EntitlementRepository) *RepositoryFactory_EntitlementRepository_Call {
	_c.Call.Return(run)
	return _c
}

func (_m *RepositoryFactory) UserRepository(db database.DBTX) interfaces.UserRepository {
	ret := _m.Called(db)

	if len(ret) == 0 {
		panic("no return value specified for UserRepository")
	}

	var r0 interfaces.UserRepository
	if rf, ok := ret.Get(0).(func(database.DBTX) interfaces.UserRepository); ok {
		r0 = rf(db)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(interfaces.UserRepository)
		}
	}

	return r0
}

type RepositoryFactory_UserRepository_Call struct {
	*mock.Call
}

func (_e *RepositoryFactory_Expecter) UserRepository(db any) *RepositoryFactory_UserRepository_Call {
	return &RepositoryFactory_UserRepository_Call{Call: _e.mock.On("UserRepository", db)}
}

func (_c *RepositoryFactory_UserRepository_Call) Run(run func(db database.DBTX)) *RepositoryFactory_UserRepository_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(database.DBTX))
	})
	return _c
}

func (_c *RepositoryFactory_UserRepository_Call) Return(_a0 interfaces.UserRepository) *RepositoryFactory_UserRepository_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *RepositoryFactory_UserRepository_Call) RunAndReturn(run func(database.DBTX) interfaces.UserRepository) *RepositoryFactory_UserRepository_Call {
	_c.Call.Return(run)
	return _c
}

func (_m *RepositoryFactory) UserIdentityRepository(db database.DBTX) interfaces.UserIdentityRepository {
	ret := _m.Called(db)

	if len(ret) == 0 {
		panic("no return value specified for UserIdentityRepository")
	}

	var r0 interfaces.UserIdentityRepository
	if rf, ok := ret.Get(0).(func(database.DBTX) interfaces.UserIdentityRepository); ok {
		r0 = rf(db)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(interfaces.UserIdentityRepository)
		}
	}

	return r0
}

type RepositoryFactory_UserIdentityRepository_Call struct {
	*mock.Call
}

func (_e *RepositoryFactory_Expecter) UserIdentityRepository(db any) *RepositoryFactory_UserIdentityRepository_Call {
	return &RepositoryFactory_UserIdentityRepository_Call{Call: _e.mock.On("UserIdentityRepository", db)}
}

func (_c *RepositoryFactory_UserIdentityRepository_Call) Run(run func(db database.DBTX)) *RepositoryFactory_UserIdentityRepository_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(database.DBTX))
	})
	return _c
}

func (_c *RepositoryFactory_UserIdentityRepository_Call) Return(_a0 interfaces.UserIdentityRepository) *RepositoryFactory_UserIdentityRepository_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *RepositoryFactory_UserIdentityRepository_Call) RunAndReturn(run func(database.DBTX) interfaces.UserIdentityRepository) *RepositoryFactory_UserIdentityRepository_Call {
	_c.Call.Return(run)
	return _c
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
