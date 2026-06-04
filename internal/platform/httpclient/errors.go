package httpclient

import "errors"

var (
	ErrObservabilityRequired = errors.New("httpclient: observability provider é obrigatório")
	ErrBaseURLRequired       = errors.New("httpclient: BaseURL é obrigatório para resolver paths relativos")
	ErrNilRequest            = errors.New("httpclient: request não pode ser nil")
)
