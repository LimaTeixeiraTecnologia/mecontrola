package interfaces

import (
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
)

var (
	ErrNicknameConflict = domain.ErrNicknameConflict
	ErrCardNotFound     = domain.ErrCardNotFound
)
