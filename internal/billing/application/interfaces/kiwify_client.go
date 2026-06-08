package interfaces

import (
	"context"
	"time"
)

type KiwifySale struct {
	ID                 string
	KiwifyProductID    string
	OrderID            string
	SubscriptionID     string
	ParentOrderID      string
	Status             string
	SaleType           string
	PaymentMethod      string
	FunnelToken        string
	CustomerEmail      string
	CustomerMobileE164 string
	OccurredAt         time.Time
	UpdatedAt          time.Time
	RefundedAt         time.Time
}

type KiwifySalePage struct {
	Sales   []KiwifySale
	HasMore bool
}

type KiwifyClient interface {
	ListSalesUpdatedSince(ctx context.Context, windowStart time.Time, windowEnd time.Time, page int) (KiwifySalePage, error)
	GetSale(ctx context.Context, saleID string) (KiwifySale, error)
}
