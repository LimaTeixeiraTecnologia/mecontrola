package services

type GatewayAuthResultKind uint8

const (
	GatewayAuthValid GatewayAuthResultKind = iota + 1
	GatewayAuthRotated
	GatewayAuthInvalidSignature
	GatewayAuthStaleTimestamp
	GatewayAuthMissingHeader
)

type GatewayAuthTimestampFailureKind uint8

const (
	GatewayAuthTimestampFailureInvalid GatewayAuthTimestampFailureKind = iota + 1
	GatewayAuthTimestampFailureStale
)

type GatewayAuthResult struct {
	Kind             GatewayAuthResultKind
	TimestampFailure GatewayAuthTimestampFailureKind
}

func (r GatewayAuthResult) IsAuthorized() bool {
	return r.Kind == GatewayAuthValid || r.Kind == GatewayAuthRotated
}
