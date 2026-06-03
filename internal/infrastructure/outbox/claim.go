package outbox

import "time"

// ClaimID é o identificador primário de uma delivery (bigserial da tabela outbox_deliveries).
type ClaimID int64

// Claim representa uma delivery que foi reivindicada pelo Dispatcher para processamento.
// Criada por Storage.ClaimReady e consumida pelo loop do Dispatcher.
type Claim struct {
	// ID é o identificador primário da delivery na tabela outbox_deliveries.
	ID ClaimID
	// Event é o evento hidratado via SELECT em outbox_events por event_id.
	Event Event
	// SubscriptionName identifica o handler responsável por esta delivery.
	SubscriptionName SubscriptionName
	// Attempt é o número de tentativas consumido até este claim (já incrementado pelo UPDATE).
	Attempt Attempt
	// ClaimedAt é o instante em que o claim foi feito.
	ClaimedAt time.Time
}
