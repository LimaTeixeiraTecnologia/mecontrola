package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
)

type ToolHandle interface {
	ID() string
	Description() string
	Parameters() map[string]any
	Invoke(ctx context.Context, argsJSON []byte) ([]byte, error)
}

type tool[I, O any] struct {
	id        string
	desc      string
	in        llm.Schema
	out       llm.Schema
	execFn    func(context.Context, I) (O, error)
	validator *jsonschema.Schema
	valErr    error
	valOnce   sync.Once
}

func NewTool[I, O any](id, desc string, in, out llm.Schema, exec func(context.Context, I) (O, error)) ToolHandle {
	return &tool[I, O]{
		id:     id,
		desc:   desc,
		in:     in,
		out:    out,
		execFn: exec,
	}
}

func (t *tool[I, O]) ID() string {
	return t.id
}

func (t *tool[I, O]) Description() string {
	return t.desc
}

func (t *tool[I, O]) Parameters() map[string]any {
	return t.in.Schema
}

func (t *tool[I, O]) compileValidator() {
	if len(t.in.Schema) == 0 {
		return
	}
	raw, err := json.Marshal(t.in.Schema)
	if err != nil {
		t.valErr = fmt.Errorf("marshal schema: %w", err)
		return
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.valErr = fmt.Errorf("decode schema: %w", err)
		return
	}
	c := jsonschema.NewCompiler()
	resource := "tool:///" + t.id
	if err := c.AddResource(resource, doc); err != nil {
		t.valErr = fmt.Errorf("add schema resource: %w", err)
		return
	}
	sch, err := c.Compile(resource)
	if err != nil {
		t.valErr = fmt.Errorf("compile schema: %w", err)
		return
	}
	t.validator = sch
}

func (t *tool[I, O]) Invoke(ctx context.Context, argsJSON []byte) ([]byte, error) {
	t.valOnce.Do(t.compileValidator)
	if t.valErr != nil {
		return nil, fmt.Errorf("tool.invoke: %w", t.valErr)
	}
	if t.validator != nil {
		instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(argsJSON))
		if err != nil {
			return nil, fmt.Errorf("tool.invoke: parse input: %w", err)
		}
		if err := t.validator.Validate(instance); err != nil {
			return nil, fmt.Errorf("tool.invoke: schema validation: %w", err)
		}
	}
	var input I
	if err := json.Unmarshal(argsJSON, &input); err != nil {
		return nil, fmt.Errorf("tool.invoke: unmarshal input: %w", err)
	}
	result, err := t.execFn(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("tool.invoke: exec: %w", err)
	}
	out, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("tool.invoke: marshal output: %w", err)
	}
	return out, nil
}
