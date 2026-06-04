package job

type OverlapPolicy int

const (
	OverlapSkip OverlapPolicy = iota + 1
	OverlapAllow
)
