package scorer

import "errors"

var (
	ErrJudgeContractNotMet = errors.New("scorer: llm judge structured output contract not satisfied")
	ErrInvalidScore        = errors.New("scorer: score must be between 0 and 1")
)
