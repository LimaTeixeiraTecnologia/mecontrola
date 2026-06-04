package postgres

import "errors"

var (
	ErrUserNotFound            = errors.New("identity: usuário não encontrado")
	ErrDuplicateWhatsAppNumber = errors.New("identity: número whatsapp já existe")
)
