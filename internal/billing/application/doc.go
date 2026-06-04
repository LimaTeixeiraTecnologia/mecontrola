// Package application contém os casos de uso do módulo billing, suas interfaces (ports)
// e os DTOs de entrada e saída.
//
// Responsabilidades:
//   - interfaces/: contratos consumidos pelos use cases e implementados pela infrastructure
//   - usecases/: orquestração de leitura, resolução de input, chamada de serviços e persistência
//   - dtos/input/: structs de entrada para os use cases
//   - dtos/output/: structs de saída para os use cases
//
// Restrição: este pacote não pode importar internal/billing/infrastructure nem nenhum driver
// concreto (pgx, net/http, kafka, etc.).
package application
