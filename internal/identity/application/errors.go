package application

import "errors"

var (
	ErrUserNotFound        = errors.New("identity: user not found")
	ErrWhatsAppNumberInUse = errors.New("identity: whatsapp number already in use")
	ErrEmailInUse          = errors.New("identity: email already in use")
	ErrEntitlementNotFound = errors.New("identity: entitlement not found")
	ErrInvalidWhatsApp     = errors.New("identity: invalid whatsapp")
	ErrInvalidEmail        = errors.New("identity: invalid email")
	ErrUnknownUser         = errors.New("identity: unknown user")
)
