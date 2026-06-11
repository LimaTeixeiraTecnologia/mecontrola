package kiwifypayload

import (
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/input"
)

func ToSaleApprovedInput(p Payload) input.ProcessSaleApprovedInput {
	token, _ := ExtractFunnel(p)
	return input.ProcessSaleApprovedInput{
		EnvelopeID:         p.OrderID,
		SaleID:             p.OrderID,
		KiwifyProductID:    p.Product.ProductID,
		OrderID:            p.OrderID,
		KiwifySubID:        p.SubscriptionID,
		FunnelToken:        token,
		CustomerMobileE164: p.Customer.Mobile,
		CustomerEmail:      p.Customer.Email,
		OccurredAt:         approvedAtUTC(p),
	}
}

func ToSubscriptionRenewedInput(p Payload) input.ProcessSubscriptionRenewedInput {
	return input.ProcessSubscriptionRenewedInput{
		EnvelopeID:      p.OrderID,
		SaleID:          p.OrderID,
		KiwifyProductID: p.Product.ProductID,
		OrderID:         p.OrderID,
		KiwifySubID:     p.SubscriptionID,
		OccurredAt:      renewalAtUTC(p),
	}
}

func ToSubscriptionLateInput(p Payload) input.ProcessSubscriptionLateInput {
	return input.ProcessSubscriptionLateInput{
		EnvelopeID:  p.OrderID,
		SaleID:      p.OrderID,
		OrderID:     p.OrderID,
		KiwifySubID: p.SubscriptionID,
		OccurredAt:  renewalAtUTC(p),
	}
}

func ToSubscriptionCanceledInput(p Payload) input.ProcessSubscriptionCanceledInput {
	return input.ProcessSubscriptionCanceledInput{
		EnvelopeID:  p.OrderID,
		SaleID:      p.OrderID,
		OrderID:     p.OrderID,
		KiwifySubID: p.SubscriptionID,
		OccurredAt:  cancellationAtUTC(p),
	}
}

func ToRefundOrChargebackInput(p Payload) input.ProcessRefundOrChargebackInput {
	return input.ProcessRefundOrChargebackInput{
		EnvelopeID: p.OrderID,
		SaleID:     p.OrderID,
		OrderID:    p.OrderID,
		Trigger:    p.WebhookEventType,
		OccurredAt: refundAtUTC(p),
	}
}

func approvedAtUTC(p Payload) time.Time {
	if !p.ApprovedDate.IsZero() {
		return p.ApprovedDate.Time
	}
	if !p.UpdatedAt.IsZero() {
		return p.UpdatedAt.Time
	}
	if p.Subscription != nil && !p.Subscription.StartDate.IsZero() {
		return p.Subscription.StartDate.Time
	}
	return time.Time{}
}

func renewalAtUTC(p Payload) time.Time {
	if !p.UpdatedAt.IsZero() {
		return p.UpdatedAt.Time
	}
	if p.Subscription != nil && !p.Subscription.NextPayment.IsZero() {
		return p.Subscription.NextPayment.Time
	}
	return time.Time{}
}

func cancellationAtUTC(p Payload) time.Time {
	if !p.UpdatedAt.IsZero() {
		return p.UpdatedAt.Time
	}
	if p.Subscription != nil && !p.Subscription.StartDate.IsZero() {
		return p.Subscription.StartDate.Time
	}
	return time.Time{}
}

func refundAtUTC(p Payload) time.Time {
	if p.RefundedAt != nil && !p.RefundedAt.IsZero() {
		return p.RefundedAt.Time
	}
	return p.UpdatedAt.Time
}
