package kiwify

import "errors"

var (
	ErrKiwifyAuth        = errors.New("billing/kiwify: falha de autenticação OAuth")
	ErrKiwifyRateLimited = errors.New("billing/kiwify: rate limit excedido")
	ErrKiwifyServer      = errors.New("billing/kiwify: erro interno do servidor Kiwify")
	ErrKiwifyBadRequest  = errors.New("billing/kiwify: requisição inválida")
)
