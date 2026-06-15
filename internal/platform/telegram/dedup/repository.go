package dedup

import "context"

type UpdateRepository interface {
	InsertIfAbsent(ctx context.Context, botID, updateID int64) (inserted bool, err error)
}
