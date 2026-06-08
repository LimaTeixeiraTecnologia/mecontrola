package meta

import "errors"

var (
	ErrMetaServer     = errors.New("onboarding/meta: erro interno do servidor Meta")
	ErrMetaBadRequest = errors.New("onboarding/meta: requisição inválida")
	ErrMetaAuth       = errors.New("onboarding/meta: autenticação inválida")
)
