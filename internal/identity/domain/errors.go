package domain

import "errors"

var (
	ErrInvalidUserID          = errors.New("identity: user id deve ser uuid v4")
	ErrUserRequiresNumber     = errors.New("identity: user requer whatsapp number válido")
	ErrUserRequiresTimestamps = errors.New("identity: user requer created_at e updated_at")
	ErrUserAlreadyDeleted     = errors.New("identity: user já está soft-deleted")
)
