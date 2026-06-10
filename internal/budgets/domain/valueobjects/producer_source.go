package valueobjects

import (
	"errors"
	"fmt"
)

var ErrProducerSourceEmpty = errors.New("budgets: producer source não pode ser vazio")

type ProducerSource struct {
	value string
}

func NewProducerSource(raw string) (ProducerSource, error) {
	if raw == "" {
		return ProducerSource{}, fmt.Errorf("budgets: %w", ErrProducerSourceEmpty)
	}
	return ProducerSource{value: raw}, nil
}

func (p ProducerSource) String() string {
	return p.value
}

func (p ProducerSource) Equal(other ProducerSource) bool {
	return p.value == other.value
}
