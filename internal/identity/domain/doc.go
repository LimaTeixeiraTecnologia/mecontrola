// Package domain contém as regras de negócio puras do módulo identity.
//
// Responsabilidades: usuário, sessão, JWT/refresh, RBAC e audit de acesso.
// Este pacote é o coração hexagonal do módulo identity e NÃO pode importar
// application, adapters, infrastructure, configs ou qualquer biblioteca de IO.
// Todo código aqui é portável, testável sem banco e sem HTTP.
package domain
