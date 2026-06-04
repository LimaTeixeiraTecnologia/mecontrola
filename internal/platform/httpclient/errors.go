package httpclient

import "errors"

var (
	// ErrObservabilityRequired indica que NewClient foi chamado sem provider de observabilidade.
	ErrObservabilityRequired = errors.New("httpclient: observability provider é obrigatório")

	// ErrBaseURLRequired indica que o cliente foi configurado sem baseURL e o caller passou um path relativo.
	ErrBaseURLRequired = errors.New("httpclient: BaseURL é obrigatório para resolver paths relativos")

	// ErrNilRequest indica que Do recebeu *http.Request nulo.
	ErrNilRequest = errors.New("httpclient: request não pode ser nil")
)
