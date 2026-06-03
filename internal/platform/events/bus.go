package events

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"sync"
	"sync/atomic"
)

// ErrBusClosed é retornado por Publish quando o Bus já foi fechado via Close.
var ErrBusClosed = errors.New("events: bus fechado")

// _defaultBufferSize é o tamanho padrão do buffer por subscriber.
const _defaultBufferSize = 100

// dispatch é o envelope enviado pelo canal de cada subscriber.
// Transporta tanto o contexto do publicador quanto o evento,
// eliminando a dependência de context.Background() no loop do handler.
type dispatch struct {
	ctx context.Context //nolint:containedctx
	evt any
}

// subscription representa um subscriber registrado para um tipo de evento.
type subscription struct {
	ch     chan dispatch
	handle func(ctx context.Context, evt any) error
	done   chan struct{}
	closed atomic.Bool
}

// safeSend tenta enviar val para s.ch de forma não-bloqueante.
// Retorna:
//   - "sent"    true  — evento entregue no buffer.
//   - "sent"    false, pânico recuperado — canal fechado (corrida TOCTOU Publish+Close).
//   - "dropped" true  — buffer cheio; evento descartado (backpressure).
//
// O recover protege contra o pânico de envio num canal já fechado,
// que pode ocorrer na janela entre a cópia da lista de subs (com RLock) e
// o envio, quando Close fecha o canal concorrentemente.
func (s *subscription) safeSend(val dispatch) (sent bool, dropped bool) {
	defer func() {
		if recover() != nil {
			sent = false
			dropped = false
		}
	}()
	select {
	case s.ch <- val:
		return true, false
	default:
		return false, true
	}
}

// Bus é o eventbus in-process tipado para o mecontrola.
// Cada subscriber recebe um canal bufferizado independente;
// eventos publicados com buffer cheio são descartados (backpressure por drop).
//
// Uso:
//
//	bus := events.NewBus(events.WithBufferSize(200))
//	unsub, _ := events.Subscribe[MyEvent](bus, func(ctx context.Context, evt MyEvent) error { ... })
//	defer unsub()
//	_ = events.Publish(bus, ctx, myEvt)
//	_ = bus.Close(ctx)
type Bus struct {
	mu         sync.RWMutex
	subs       map[reflect.Type][]*subscription
	closed     atomic.Bool
	bufferSize int
	wg         sync.WaitGroup
}

// BusOption configura um Bus.
type BusOption func(*Bus)

// WithBufferSize define o tamanho do buffer por subscriber.
// Deve ser positivo; valores ≤ 0 são ignorados.
func WithBufferSize(n int) BusOption {
	return func(b *Bus) {
		if n > 0 {
			b.bufferSize = n
		}
	}
}

// NewBus cria um Bus pronto para uso com as opções fornecidas.
func NewBus(opts ...BusOption) *Bus {
	b := &Bus{
		subs:       make(map[reflect.Type][]*subscription),
		bufferSize: _defaultBufferSize,
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Publish envia o evento evt para todos os subscribers registrados para o tipo E.
// Retorna ErrBusClosed se o bus já foi fechado.
// Se o canal de um subscriber estiver cheio, o evento é descartado e logado
// com incremento da contagem de drops — sem bloquear o publicador.
func Publish[E Event](b *Bus, ctx context.Context, evt E) error {
	if b.closed.Load() {
		return ErrBusClosed
	}

	t := reflect.TypeFor[E]()

	b.mu.RLock()
	subs := make([]*subscription, len(b.subs[t]))
	copy(subs, b.subs[t])
	b.mu.RUnlock()

	d := dispatch{ctx: ctx, evt: evt}
	for _, sub := range subs {
		sent, dropped := sub.safeSend(d)
		if dropped {
			slog.WarnContext(ctx, "events: buffer cheio, evento descartado",
				slog.String("event_name", fmt.Sprintf("%v", evt.Name())),
				slog.String("reason", "buffer_full"),
			)
		}
		_ = sent
	}
	return nil
}

// Subscribe registra handler para receber eventos do tipo E.
// Retorna uma função unsubscribe e um erro.
// O handler é executado em goroutine dedicada por subscriber.
// Retorna erro se o bus já estiver fechado.
func Subscribe[E Event](b *Bus, handler func(ctx context.Context, evt E) error) (unsubscribe func(), err error) {
	if b.closed.Load() {
		return func() {}, ErrBusClosed
	}

	t := reflect.TypeFor[E]()

	sub := &subscription{
		ch:   make(chan dispatch, b.bufferSize),
		done: make(chan struct{}),
		handle: func(ctx context.Context, raw any) error {
			evt, ok := raw.(E)
			if !ok {
				return fmt.Errorf("events: tipo inesperado %T para subscriber de %s", raw, t)
			}
			return handler(ctx, evt)
		},
	}

	b.mu.Lock()
	b.subs[t] = append(b.subs[t], sub)
	b.mu.Unlock()

	b.wg.Go(func() {
		defer close(sub.done)
		// Cada dispatch carrega o contexto do publicador, propagando deadline/cancel
		// ao handler em vez de usar context.Background().
		for d := range sub.ch {
			if err := sub.handle(d.ctx, d.evt); err != nil {
				slog.ErrorContext(d.ctx, "events: erro no handler de subscriber",
					slog.String("event_type", t.String()),
					slog.String("error", err.Error()),
				)
			}
		}
	})

	unsubscribe = func() {
		if !sub.closed.CompareAndSwap(false, true) {
			// Já foi fechado por Close() ou por chamada anterior.
			<-sub.done
			return
		}
		b.mu.Lock()
		list := b.subs[t]
		for i, s := range list {
			if s == sub {
				b.subs[t] = append(list[:i], list[i+1:]...)
				break
			}
		}
		b.mu.Unlock()
		close(sub.ch)
		<-sub.done
	}

	return unsubscribe, nil
}

// Close encerra o bus de forma idempotente.
// Fecha os canais de todos os subscribers, aguardando drenagem até que
// o contexto seja cancelado ou todos os handlers terminem.
// Novos Publish após Close retornam ErrBusClosed.
// Chamar Close mais de uma vez é seguro e retorna nil.
func (b *Bus) Close(ctx context.Context) error {
	if !b.closed.CompareAndSwap(false, true) {
		return nil
	}

	b.mu.Lock()
	var allSubs []*subscription
	for _, subs := range b.subs {
		allSubs = append(allSubs, subs...)
	}
	// Limpa o mapa para que unsubscribe chamados após Close não re-fechem canais.
	b.subs = make(map[reflect.Type][]*subscription)
	b.mu.Unlock()

	for _, sub := range allSubs {
		if sub.closed.CompareAndSwap(false, true) {
			close(sub.ch)
		}
	}

	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("events: timeout ao fechar bus: %w", ctx.Err())
	}
}
