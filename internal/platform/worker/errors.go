package worker

import "errors"

var (
	errDuplicateName = errors.New("worker: nome duplicado")
	errStopTimeout   = errors.New("worker: timeout de shutdown excedido")
)
