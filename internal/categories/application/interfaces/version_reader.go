package interfaces

import "context"

type VersionReader interface {
	Current(ctx context.Context) (int64, error)
}
