package input

import "errors"

var (
	ErrRawBodyRequired         = errors.New("raw_body: obrigatório")
	ErrSignatureStatusRequired = errors.New("signature_status: obrigatório")
	ErrSaleIDRequired          = errors.New("sale_id: obrigatório")
	ErrOrderIDRequired         = errors.New("order_id: obrigatório")
	ErrKiwifySubIDRequired     = errors.New("kiwify_sub_id: obrigatório")
	ErrKiwifyProductIDRequired = errors.New("kiwify_product_id: obrigatório")
	ErrOccurredAtRequired      = errors.New("occurred_at: obrigatório")
	ErrWindowStartRequired     = errors.New("window_start: obrigatório")
	ErrWindowEndRequired       = errors.New("window_end: obrigatório")
	ErrWindowEndBeforeStart    = errors.New("window_end: deve ser posterior a window_start")
	ErrEventTypeRequired       = errors.New("event_type: obrigatório")
	ErrPayloadRequired         = errors.New("payload: obrigatório")
)
