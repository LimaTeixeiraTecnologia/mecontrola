package dedup

import "context"

type MessageRepository interface {
	InsertIfAbsent(ctx context.Context, wamid string) (inserted bool, err error)
}
