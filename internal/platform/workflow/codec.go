package workflow

import (
	"encoding/json"
	"fmt"
)

type Codec[S any] struct{}

func NewCodec[S any]() Codec[S] {
	return Codec[S]{}
}

func (c Codec[S]) Encode(state S) ([]byte, error) {
	b, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("workflow: codec: encode: %w", err)
	}
	return b, nil
}

func (c Codec[S]) Decode(data []byte) (S, error) {
	var s S
	if err := json.Unmarshal(data, &s); err != nil {
		return s, fmt.Errorf("workflow: codec: decode: %w", err)
	}
	return s, nil
}
