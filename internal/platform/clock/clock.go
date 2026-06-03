// Package clock fornece a interface Clock e implementações para uso em produção e testes.
// A injeção de Clock garante determinismo nos testes que dependem de tempo.
package clock

import "time"

// Clock abstrai a obtenção do instante atual.
// Deve ser injetado em qualquer componente que precise de time.Now() para
// garantir determinismo em testes.
type Clock interface {
	// Now retorna o instante atual no fuso UTC.
	Now() time.Time
}

// SystemClock é a implementação de Clock para uso em produção.
// Delega diretamente para time.Now().
type SystemClock struct{}

// Now retorna time.Now() em UTC.
func (SystemClock) Now() time.Time {
	return time.Now().UTC()
}

// NewSystemClock cria uma nova instância de SystemClock.
func NewSystemClock() SystemClock {
	return SystemClock{}
}
