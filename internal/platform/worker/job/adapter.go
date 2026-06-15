package job

import (
	"context"
	"time"
)

type Adapter struct {
	name          string
	schedule      string
	run           func(context.Context) error
	overlapPolicy OverlapPolicy
	timeout       time.Duration
}

func NewAdapter(name, schedule string, run func(context.Context) error) *Adapter {
	return &Adapter{
		name:          name,
		schedule:      schedule,
		run:           run,
		overlapPolicy: OverlapSkip,
	}
}

func NewAdapterWithPolicy(name, schedule string, run func(context.Context) error, policy OverlapPolicy) *Adapter {
	return &Adapter{
		name:          name,
		schedule:      schedule,
		run:           run,
		overlapPolicy: policy,
	}
}

func NewAdapterWithTimeout(name, schedule string, run func(context.Context) error, timeout time.Duration) *Adapter {
	return &Adapter{
		name:          name,
		schedule:      schedule,
		run:           run,
		overlapPolicy: OverlapSkip,
		timeout:       timeout,
	}
}

func (a *Adapter) Name() string                  { return a.name }
func (a *Adapter) Schedule() string              { return a.schedule }
func (a *Adapter) Run(ctx context.Context) error { return a.run(ctx) }
func (a *Adapter) OverlapPolicy() OverlapPolicy  { return a.overlapPolicy }
func (a *Adapter) Timeout() time.Duration        { return a.timeout }
