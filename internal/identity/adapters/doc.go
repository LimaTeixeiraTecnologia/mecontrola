// Package adapters implementa os ports declarados em application para o módulo identity.
//
// Responsabilidades: implementações concretas de Repository (pgx/Postgres),
// EventPublisher (eventbus in-process), handlers HTTP (Chi) e demais adaptadores
// de entrada/saída. Este pacote PODE importar domain, application e infrastructure.
package adapters
