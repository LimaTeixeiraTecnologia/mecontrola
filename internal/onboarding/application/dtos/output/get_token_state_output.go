package output

type GetTokenStateOutput struct {
	ReadyToActivate  bool
	WaMeURL          string
	TelegramDeepLink string
	BotNumberDisplay string
	Reason           string
	SupportURL       string
}
