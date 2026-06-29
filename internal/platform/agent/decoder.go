package agent

import "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"

type StructuredDecoder interface {
	Schema() llm.Schema
	Validate(raw []byte) error
}

type contractAdapter[T any] struct {
	contract llm.StructuredContract[T]
}

func NewDecoder[T any](c llm.StructuredContract[T]) StructuredDecoder {
	return &contractAdapter[T]{contract: c}
}

func (a *contractAdapter[T]) Schema() llm.Schema {
	return a.contract.Schema()
}

func (a *contractAdapter[T]) Validate(raw []byte) error {
	_, err := a.contract.Decode(raw)
	return err
}
