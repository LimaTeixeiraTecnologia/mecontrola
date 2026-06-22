package input

import "errors"

var (
	ErrFromE164Required       = errors.New("from_e164: obrigatório")
	ErrActivationPathRequired = errors.New("activation_path: obrigatório")
	ErrPlanIDRequired         = errors.New("plan_id: obrigatório")
	ErrExternalSaleIDRequired = errors.New("external_sale_id: obrigatório")
	ErrPaidAtRequired         = errors.New("paid_at: obrigatório")
	ErrSubscriptionIDRequired = errors.New("subscription_id: obrigatório")
	ErrFunnelTokenRequired    = errors.New("funnel_token: obrigatório")
	ErrTokenRequired          = errors.New("token: obrigatório")
)
