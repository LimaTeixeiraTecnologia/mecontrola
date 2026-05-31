// Package adapters implementa os ports declarados em application para o módulo notifications.
//
// Responsabilidades: implementações concretas de NotificationRepository (Postgres),
// DeliveryPort (WhatsApp/Meta Cloud API), handlers HTTP de gerenciamento de
// notificações e adaptadores de eventbus. Este pacote PODE importar domain,
// application e infrastructure.
package adapters
