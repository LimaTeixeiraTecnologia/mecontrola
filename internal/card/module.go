package card

import (
	"context"
	"fmt"
	"net/http"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/services"
	httpserver "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/http/server"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/http/server/handlers"
	cardjobs "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/jobs/handlers"
	cardconsumers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/messaging/database/consumers"
	cardproducers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/messaging/database/producers"
	cardrepo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/idempotency"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/notification"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/ratelimit"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/worker"
)

type EventHandlerRegistration struct {
	EventType string
	Handler   events.Handler
}

type CardModule struct {
	RepositoryFactory   interfaces.RepositoryFactory
	CardRouter          *httpserver.CardRouter
	CardLookup          *usecases.GetCardForUser
	ListCardsUC         *usecases.ListCards
	CreateCardUC        *usecases.CreateCard
	GetCardUC           *usecases.GetCard
	UpdateCardUC        *usecases.UpdateCard
	UpdateCardLimitUC   *usecases.UpdateCardLimit
	SoftDeleteCardUC    *usecases.SoftDeleteCard
	InvoiceForUC        *usecases.InvoiceFor
	InvoiceDueAlertsJob worker.Job
	EventHandlers       []EventHandlerRegistration
}

func NewCardModule(
	ctx context.Context,
	cfg *configs.Config,
	o11y observability.Observability,
	mgr manager.Manager,
	gatewayAuth func(http.Handler) http.Handler,
	channelGateway notification.ChannelGateway,
	channelResolver interfaces.UserChannelResolver,
) (CardModule, error) {
	loc, err := services.NewSaoPauloLocation()
	if err != nil {
		return CardModule{}, fmt.Errorf("card.module: %w", err)
	}

	factory := cardrepo.NewRepositoryFactory(o11y)
	idemStorage := idempotency.NewPostgresStorage(mgr)

	createUoW := uow.New[entities.Card](mgr)
	updateUoW := uow.New[entities.Card](mgr)
	updateLimitUoW := uow.New[entities.Card](mgr)
	deleteUoW := uow.New[struct{}](mgr)

	createCard := usecases.NewCreateCard(createUoW, factory, idemStorage, o11y)
	getCard := usecases.NewGetCard(factory, mgr, o11y)
	listCards := usecases.NewListCards(factory, mgr, o11y)
	updateCard := usecases.NewUpdateCard(updateUoW, factory, idemStorage, o11y)
	updateCardLimit := usecases.NewUpdateCardLimit(updateLimitUoW, factory, idemStorage, o11y)
	softDelete := usecases.NewSoftDeleteCard(deleteUoW, factory, idemStorage, o11y)
	invoiceFor := usecases.NewInvoiceFor(factory, mgr, loc, o11y)
	getCardForUser := usecases.NewGetCardForUser(factory, mgr, o11y)

	createHandler := handlers.NewCreateCardHandler(createCard, o11y)
	listHandler := handlers.NewListCardsHandler(listCards, o11y)
	getHandler := handlers.NewGetCardHandler(getCard, o11y)
	updateHandler := handlers.NewUpdateCardHandler(updateCard, o11y)
	updateLimitHandler := handlers.NewUpdateCardLimitHandler(updateCardLimit, o11y)
	deleteHandler := handlers.NewDeleteCardHandler(softDelete, o11y)
	invoiceForHandler := handlers.NewInvoiceForHandler(invoiceFor, o11y)

	userRateLimit := ratelimit.NewRateLimitMiddleware(ctx, ratelimit.RateLimitConfig{
		PerMinute: cfg.AuthRateLimit.PerUserPerMin,
		Burst:     cfg.AuthRateLimit.PerUserBurst,
		Extractor: ratelimit.ByUserID,
		Scope:     "user",
	}, o11y)

	router := httpserver.NewCardRouter(createHandler, listHandler, getHandler, updateHandler, updateLimitHandler, deleteHandler, invoiceForHandler, idemStorage, o11y, gatewayAuth, userRateLimit)

	onboardingCardConsumer := cardconsumers.NewOnboardingCardConsumer(createCard, o11y)

	eventHandlers := []EventHandlerRegistration{
		{EventType: "onboarding.card_registered", Handler: onboardingCardConsumer},
	}

	var invoiceDueAlertsJob worker.Job
	if channelGateway != nil && channelResolver != nil {
		outboxFactory := outbox.NewRepositoryFactory(o11y)
		invoiceDuePublisher := cardproducers.NewInvoiceDuePublisher(outboxFactory, cfg.OutboxConfig, id.NewUUIDGenerator(), o11y)
		evaluateUoW := uow.NewVoid(mgr, uow.WithObservability(o11y))
		evaluateInvoiceDue := usecases.NewEvaluateInvoiceDueAlerts(
			factory,
			invoiceDuePublisher,
			evaluateUoW,
			loc,
			cfg.CardConfig.InvoiceDueWindowDays,
			cfg.CardConfig.InvoiceDueScanLimit,
			o11y,
		)
		notifyInvoiceDue := usecases.NewNotifyInvoiceDue(mgr, factory, channelResolver, channelGateway, loc, o11y)
		invoiceDueNotifier := cardconsumers.NewInvoiceDueNotifier(notifyInvoiceDue, o11y)
		eventHandlers = append(eventHandlers, EventHandlerRegistration{
			EventType: "card.invoice_due.v1",
			Handler:   invoiceDueNotifier,
		})
		if cfg.CardConfig.InvoiceDueAlertsEnabled {
			invoiceDueAlertsJob = cardjobs.NewInvoiceDueAlertsJob(evaluateInvoiceDue, cfg.CardConfig)
		}
	}

	return CardModule{
		RepositoryFactory:   factory,
		CardRouter:          router,
		CardLookup:          getCardForUser,
		ListCardsUC:         listCards,
		CreateCardUC:        createCard,
		GetCardUC:           getCard,
		UpdateCardUC:        updateCard,
		UpdateCardLimitUC:   updateCardLimit,
		SoftDeleteCardUC:    softDelete,
		InvoiceForUC:        invoiceFor,
		InvoiceDueAlertsJob: invoiceDueAlertsJob,
		EventHandlers:       eventHandlers,
	}, nil
}
