package kiwify

import "time"

type oauthTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type salesListResponse struct {
	Data        []saleResponse `json:"data"`
	TotalItems  int            `json:"totalItems"`
	CurrentPage int            `json:"currentPage"`
	HasMore     bool           `json:"hasMore"`
}

type saleResponse struct {
	ID             string           `json:"id"`
	Reference      string           `json:"reference"`
	Status         string           `json:"status"`
	SaleType       string           `json:"sale_type"`
	PaymentMethod  string           `json:"payment_method"`
	NetAmount      float64          `json:"net_amount"`
	ParentOrderID  string           `json:"parent_order_id"`
	SubscriptionID string           `json:"subscription_id"`
	ProductID      string           `json:"product_id"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
	RefundedAt     *time.Time       `json:"refunded_at"`
	Customer       customerResponse `json:"customer"`
	Tracking       trackingResponse `json:"tracking"`
}

type customerResponse struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"mobile"`
}

type trackingResponse struct {
	UTMSource   string `json:"utm_source"`
	UTMMedium   string `json:"utm_medium"`
	UTMCampaign string `json:"utm_campaign"`
	UTMContent  string `json:"utm_content"`
	UTMTerm     string `json:"utm_term"`
	SCK         string `json:"sck"`
	Src         string `json:"src"`
	S1          string `json:"s1"`
	S2          string `json:"s2"`
	S3          string `json:"s3"`
}
