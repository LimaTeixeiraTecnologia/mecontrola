package input

type UpsertUserByWhatsApp struct {
	WhatsAppNumber string
	Email          string
	DisplayName    string
}

func (i *UpsertUserByWhatsApp) Validate() error {
	if i.WhatsAppNumber == "" {
		return ErrWhatsAppNumberRequired
	}
	return nil
}
