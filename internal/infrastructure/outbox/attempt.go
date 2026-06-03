package outbox

// Attempt é o value object que encapsula o número de tentativas de entrega (OC #3).
// Usa uint8 para limitar o domínio a [0, 255] — o máximo configurável é 15 (D-03).
type Attempt struct {
	value uint8
}

// NewAttempt cria um Attempt a partir de um uint8.
func NewAttempt(value uint8) Attempt {
	return Attempt{value: value}
}

// Value retorna o valor numérico do attempt.
func (a Attempt) Value() uint8 { return a.value }

// Next retorna um novo Attempt incrementado em 1.
// Não faz overflow — se value == 255, permanece 255.
func (a Attempt) Next() Attempt {
	if a.value == 255 {
		return a
	}
	return Attempt{value: a.value + 1}
}

// IsExhausted retorna true quando este Attempt atingiu ou superou o limite máximo.
func (a Attempt) IsExhausted(max Attempt) bool {
	return a.value >= max.value
}
