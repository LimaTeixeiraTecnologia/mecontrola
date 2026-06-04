package worker

import "context"

type Job interface {
	Name() string
	Schedule() string
	Run(context.Context) error
}

type Consumer interface {
	Name() string
	Technology() string
	Start(context.Context) error
	Stop(context.Context) error
}
