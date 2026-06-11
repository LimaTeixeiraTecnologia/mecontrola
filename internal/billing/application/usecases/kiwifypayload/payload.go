package kiwifypayload

import (
	"encoding/json"
	"errors"
	"fmt"
)

var ErrInvalidPayload = errors.New("kiwifypayload: invalid payload")

type Payload struct {
	OrderID          string `json:"order_id"`
	OrderRef         string `json:"order_ref"`
	OrderStatus      string `json:"order_status"`
	WebhookEventType string `json:"webhook_event_type"`
	SubscriptionID   string `json:"subscription_id"`

	AbandonedID     string `json:"id"`
	AbandonedStatus string `json:"status"`

	Product            product       `json:"Product"`
	Customer           customer      `json:"Customer"`
	Subscription       *subscription `json:"Subscription"`
	TrackingParameters tracking      `json:"TrackingParameters"`

	RefundedAt   *Time `json:"refunded_at"`
	ApprovedDate Time  `json:"approved_date"`
	UpdatedAt    Time  `json:"updated_at"`
	CreatedAt    Time  `json:"created_at"`
}

type product struct {
	ProductID   string `json:"product_id"`
	ProductName string `json:"product_name"`
}

type customer struct {
	Email  string `json:"email"`
	Mobile string `json:"mobile"`
	CPF    string `json:"CPF"`
}

type subscription struct {
	StartDate   Time   `json:"start_date"`
	NextPayment Time   `json:"next_payment"`
	Status      string `json:"status"`
}

type tracking struct {
	SCK string `json:"sck"`
	S1  string `json:"s1"`
	Src string `json:"src"`
}

func Decode(raw []byte) (Payload, error) {
	var p Payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return Payload{}, errors.Join(
			ErrInvalidPayload,
			fmt.Errorf("kiwifypayload.Decode: %w", err),
		)
	}
	return p, nil
}

func (p Payload) EnvelopeID() string {
	if p.OrderID != "" {
		return p.OrderID
	}
	return p.AbandonedID
}
