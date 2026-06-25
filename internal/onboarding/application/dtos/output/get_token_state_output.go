package output

type GetTokenStateOutput struct {
	ReadyToActivate  bool
	WaMeURL          string
	BotNumberDisplay string
	Reason           string
	SupportURL       string
}
