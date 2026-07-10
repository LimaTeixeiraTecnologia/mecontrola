package domain

import "fmt"

type AuthResolvePath string

const (
	AuthResolvePathIdentity AuthResolvePath = "identity"
	AuthResolvePathLegacy   AuthResolvePath = "legacy"
	AuthResolvePathBackfill AuthResolvePath = "backfill"
)

func (p AuthResolvePath) IsValid() bool {
	switch p {
	case AuthResolvePathIdentity, AuthResolvePathLegacy, AuthResolvePathBackfill:
		return true
	default:
		return false
	}
}

func (p AuthResolvePath) String() string {
	return string(p)
}

func ParseAuthResolvePath(raw string) (AuthResolvePath, error) {
	path := AuthResolvePath(raw)
	if !path.IsValid() {
		return "", fmt.Errorf("auth_resolve_path: valor invalido %q", raw)
	}
	return path, nil
}
