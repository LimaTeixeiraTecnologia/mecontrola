package outbox

import "errors"

// ErrPermanent sinaliza falha terminal do handler. O Dispatcher transita imediatamente
// para DLQ sem consumir tentativas. O handler deve retornar wrappando este sentinel:
//
//	fmt.Errorf("schema v2 incompatível: %w", outbox.ErrPermanent)
var ErrPermanent = errors.New("outbox: erro permanente")

// ErrHandlerNotRegistered é retornado por Publisher.Publish quando o event_type informado
// não possui nenhum handler registrado no Registry.
var ErrHandlerNotRegistered = errors.New("outbox: nenhum handler registrado para event_type")

// ErrDispatcherDisabled é retornado por operações dependentes do loop de polling quando
// OUTBOX_DISPATCHER_ENABLED=false.
var ErrDispatcherDisabled = errors.New("outbox: dispatcher desabilitado")

// ErrDuplicateSubscription é retornado por Registry.Register quando o par (Name, EventType)
// já estiver registrado (D-09).
var ErrDuplicateSubscription = errors.New("outbox: subscription duplicada (name, event_type)")

// ErrInvalidEvent é retornado por NewEvent quando as invariantes do construtor falharem.
var ErrInvalidEvent = errors.New("outbox: evento invalido")
