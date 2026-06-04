package services

import "errors"

var ErrIllegalTransition = errors.New("billing: transição de estado ilegal")
