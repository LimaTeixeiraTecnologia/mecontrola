// Package domain contém as regras de negócio puras do módulo notifications.
//
// Responsabilidades: notificação, preferências de entrega, templates de mensagem,
// agendamento de alertas e lembretes. Este pacote é o coração hexagonal do
// módulo notifications e NÃO pode importar application, adapters, infrastructure,
// configs ou qualquer biblioteca de IO. Todo código aqui é portável e testável
// sem banco, sem HTTP e sem canal de entrega externo.
package domain
