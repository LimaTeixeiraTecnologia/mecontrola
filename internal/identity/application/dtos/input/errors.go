package input

import "errors"

var (
	ErrIDRequired             = errors.New("id: obrigatório ou UUID inválido")
	ErrPayloadRequired        = errors.New("payload: obrigatório")
	ErrEventTypeRequired      = errors.New("event_type: obrigatório")
	ErrExternalIDRequired     = errors.New("external_id: obrigatório")
	ErrUserIDRequired         = errors.New("user_id: obrigatório")
	ErrDisplayNameRequired    = errors.New("display_name: obrigatório")
	ErrWhatsAppNumberRequired = errors.New("whatsapp_number: obrigatório")
	ErrChannelRequired        = errors.New("channel: obrigatório")
	ErrReasonRequired         = errors.New("reason: obrigatório")
)
