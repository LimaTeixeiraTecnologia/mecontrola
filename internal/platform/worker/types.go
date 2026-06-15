package worker

import (
	"context"
	"time"
)

type Job interface {
	Name() string
	Schedule() string
	Run(context.Context) error
	Timeout() time.Duration
}

type Consumer interface {
	Name() string
	Technology() string
	Start(context.Context) error
	Stop(context.Context) error
}
