package workflow

import (
	"encoding/json"
	"fmt"
	"maps"
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

func (c Codec[S]) MergePatch(base, patch []byte) ([]byte, error) {
	var baseMap map[string]any
	if err := json.Unmarshal(base, &baseMap); err != nil {
		return nil, fmt.Errorf("workflow: codec: merge base: %w", err)
	}
	var patchMap map[string]any
	if err := json.Unmarshal(patch, &patchMap); err != nil {
		return nil, fmt.Errorf("workflow: codec: merge patch: %w", err)
	}
	merged := mergeMaps(baseMap, patchMap)
	out, err := json.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("workflow: codec: merge marshal: %w", err)
	}
	return out, nil
}

func mergeMaps(base, patch map[string]any) map[string]any {
	result := make(map[string]any, len(base))
	maps.Copy(result, base)
	for k, v := range patch {
		if v == nil {
			delete(result, k)
			continue
		}
		baseVal, baseIsMap := result[k].(map[string]any)
		patchVal, patchIsMap := v.(map[string]any)
		if baseIsMap && patchIsMap {
			result[k] = mergeMaps(baseVal, patchVal)
			continue
		}
		_, baseIsArray := result[k].([]any)
		_, patchIsArray := v.([]any)
		if baseIsArray || patchIsArray {
			result[k] = v
			continue
		}
		result[k] = v
	}
	return result
}
