package option

type Option[T any] struct {
	value   T
	present bool
}

func Some[T any](v T) Option[T] {
	return Option[T]{value: v, present: true}
}

func None[T any]() Option[T] {
	return Option[T]{}
}

func (o Option[T]) Get() (T, bool) {
	return o.value, o.present
}

func (o Option[T]) IsPresent() bool {
	return o.present
}
