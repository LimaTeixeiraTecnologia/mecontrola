package mocks

import mock "github.com/stretchr/testify/mock"

type ProcessRefundOrChargeback = processRefundOrChargeback
type ProcessSaleApproved = processSaleApproved
type ProcessSubscriptionCanceled = processSubscriptionCanceled
type ProcessSubscriptionLate = processSubscriptionLate
type ProcessSubscriptionRenewed = processSubscriptionRenewed

func NewProcessRefundOrChargeback(t interface {
	mock.TestingT
	Cleanup(func())
}) *ProcessRefundOrChargeback {
	return newProcessRefundOrChargeback(t)
}

func NewProcessSaleApproved(t interface {
	mock.TestingT
	Cleanup(func())
}) *ProcessSaleApproved {
	return newProcessSaleApproved(t)
}

func NewProcessSubscriptionCanceled(t interface {
	mock.TestingT
	Cleanup(func())
}) *ProcessSubscriptionCanceled {
	return newProcessSubscriptionCanceled(t)
}

func NewProcessSubscriptionLate(t interface {
	mock.TestingT
	Cleanup(func())
}) *ProcessSubscriptionLate {
	return newProcessSubscriptionLate(t)
}

func NewProcessSubscriptionRenewed(t interface {
	mock.TestingT
	Cleanup(func())
}) *ProcessSubscriptionRenewed {
	return newProcessSubscriptionRenewed(t)
}
