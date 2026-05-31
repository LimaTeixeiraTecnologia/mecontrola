// Package adapters implementa os ports declarados em application para o módulo finance.
//
// Responsabilidades: implementações concretas de TransactionRepository (Postgres),
// CategoryRepository, handlers HTTP de movimentações e adaptadores de eventbus.
// Este pacote PODE importar domain, application e infrastructure.
package adapters
