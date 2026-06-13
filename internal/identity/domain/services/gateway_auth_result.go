package services

type GatewayAuthResultKind uint8

const (
	GatewayAuthValid GatewayAuthResultKind = iota + 1
	GatewayAuthRotated
	GatewayAuthInvalidSignature
	GatewayAuthStaleTimestamp
	GatewayAuthInvalidTimestamp
	GatewayAuthMissingHeader
)

type GatewayAuthResult struct {
	Kind GatewayAuthResultKind
}

func (r GatewayAuthResult) IsAuthorized() bool {
	return r.Kind == GatewayAuthValid || r.Kind == GatewayAuthRotated
}
