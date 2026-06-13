package input

type RecordGatewayAuthFailureInput struct {
	UserIDRaw   string
	Reason      string
	RequestID   string
	ClientIPRaw string
}
