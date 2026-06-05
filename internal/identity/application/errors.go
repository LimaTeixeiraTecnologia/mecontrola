package application

import "errors"

var (
	ErrUserNotFound        = errors.New("identity: user not found")
	ErrWhatsAppNumberInUse = errors.New("identity: whatsapp number already in use")
	ErrEmailInUse          = errors.New("identity: email already in use")
)
