/*
Package events fornece o eventbus in-process tipado via generics Go 1.26
para comunicação assíncrona entre módulos do mecontrola.

# Declaração de um evento de domínio

Para declarar um evento, implemente a interface [Event]:

	type MessageReceived struct {
		id          events.EventID
		aggregateID string
		occurredAt  time.Time
		payload     string
	}

	func NewMessageReceived(clock clock.Clock, aggregateID, payload string) (MessageReceived, error) {
		id, err := events.NewEventID(generateULID(clock))
		if err != nil {
			return MessageReceived{}, fmt.Errorf("criar evento MessageReceived: %w", err)
		}
		return MessageReceived{
			id:          id,
			aggregateID: aggregateID,
			occurredAt:  clock.Now(),
			payload:     payload,
		}, nil
	}

	func (e MessageReceived) Name() events.EventName {
		n, _ := events.NewEventName("conversation.message-received")
		return n
	}
	func (e MessageReceived) OccurredAt() time.Time { return e.occurredAt }
	func (e MessageReceived) AggregateID() string   { return e.aggregateID }

# Publicação e subscrição

	bus := events.NewBus(events.WithBufferSize(200))

	unsub, err := events.Subscribe[MessageReceived](bus, func(ctx context.Context, evt MessageReceived) error {
		slog.InfoContext(ctx, "mensagem recebida", slog.String("aggregate_id", evt.AggregateID()))
		return nil
	})
	if err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}
	defer unsub()

	if err := events.Publish(bus, ctx, evt); err != nil {
		return fmt.Errorf("publicar evento: %w", err)
	}

# Encerramento

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := bus.Close(shutdownCtx); err != nil {
		slog.Warn("eventos: timeout ao fechar bus", slog.String("error", err.Error()))
	}

# Backpressure

Cada subscriber tem um canal bufferizado (padrão 100 slots).
Quando o buffer está cheio, o evento é descartado silenciosamente com log de warning.
Não há bloqueio do publicador — o mecanismo é drop-on-full, adequado para
eventos de observabilidade e notificações não críticas.

Para eventos críticos (ex: comandos transacionais), prefira publicação síncrona
dentro da UnitOfWork em vez do eventbus.

# Referência

ADR-003: Eventbus tipado via generics + emissão pós-UoW.Commit.
*/
package events
