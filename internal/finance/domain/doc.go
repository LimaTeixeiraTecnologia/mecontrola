// Package domain contém as regras de negócio puras do módulo finance.
//
// Responsabilidades: movimentações financeiras, categorias, metas, saldos e
// regras de orçamento pessoal. Este pacote é o coração hexagonal do módulo
// finance e NÃO pode importar application, infrastructure, configs
// ou qualquer biblioteca de IO. Todo código aqui é portável e testável sem
// banco e sem HTTP.
package domain
