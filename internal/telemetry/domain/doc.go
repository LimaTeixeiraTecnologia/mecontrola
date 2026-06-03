// Package domain contém as regras de negócio puras do módulo telemetry.
//
// Responsabilidades: eventos de telemetria de domínio, métricas de uso do produto,
// rastreamento de jornada do usuário e análise de comportamento conversacional.
// Este pacote é o coração hexagonal do módulo telemetry e NÃO pode importar
// application, infrastructure, configs ou qualquer biblioteca de IO.
// Todo código aqui é portável e testável sem banco e sem HTTP.
package domain
