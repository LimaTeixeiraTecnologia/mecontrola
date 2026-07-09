package card

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/jmoiron/sqlx"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/usecases"
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
	RepositoryFactory       interfaces.RepositoryFactory
	CardRouter              *httpserver.CardRouter
	CardLookup              *usecases.GetCardForUser
	ListCardsUC             *usecases.ListCards
	CountCardsUC            *usecases.CountCards
	CreateCardUC            *usecases.CreateCard
	GetCardUC               *usecases.GetCard
	ResolveCardByNicknameUC *usecases.ResolveCardByNickname
	UpdateCardUC            *usecases.UpdateCard
	SoftDeleteCardUC        *usecases.SoftDeleteCard
	InvoiceForUC            *usecases.InvoiceFor
	BestPurchaseDayUC       *usecases.BestPurchaseDay
	IsBankRecognizedUC      *usecases.IsBankRecognized
	InvoiceDueAlertsJob     worker.Job
	EventHandlers           []EventHandlerRegistration
}

func NewCardModule(
	ctx context.Context,
	cfg *configs.Config,
	o11y observability.Observability,
	db *sqlx.DB,
	gatewayAuth func(http.Handler) http.Handler,
	channelGateway notification.ChannelGateway,
	channelResolver interfaces.UserChannelResolver,
) (CardModule, error) {
	loc, err := services.NewSaoPauloLocation()
	if err != nil {
		return CardModule{}, fmt.Errorf("card.module: %w", err)
	}

	factory := cardrepo.NewRepositoryFactory(o11y)
	cardRepo := factory.CardRepository(db)
	idemStorage := idempotency.NewPostgresStorage(db)

	createUoW := uow.NewUnitOfWork(db)
	updateUoW := uow.NewUnitOfWork(db)
	deleteUoW := uow.NewUnitOfWork(db)

	createCard := usecases.NewCreateCard(createUoW, factory, idemStorage, o11y)
	getCard := usecases.NewGetCard(cardRepo, o11y)
	resolveCardByNickname := usecases.NewResolveCardByNickname(cardRepo, o11y)
	listCards := usecases.NewListCards(cardRepo, o11y)
	countCards := usecases.NewCountCards(cardRepo, o11y)
	updateCard := usecases.NewUpdateCard(updateUoW, factory, idemStorage, o11y)
	softDelete := usecases.NewSoftDeleteCard(deleteUoW, factory, idemStorage, o11y)
	invoiceFor := usecases.NewInvoiceFor(cardRepo, loc, o11y)
	getCardForUser := usecases.NewGetCardForUser(cardRepo, o11y)
	bestPurchaseDay := usecases.NewBestPurchaseDay(factory, db, o11y)
	isBankRecognized := usecases.NewIsBankRecognized(factory, db, o11y)

	createHandler := handlers.NewCreateCardHandler(createCard, o11y)
	listHandler := handlers.NewListCardsHandler(listCards, o11y)
	getHandler := handlers.NewGetCardHandler(getCard, o11y)
	updateHandler := handlers.NewUpdateCardHandler(updateCard, o11y)
	deleteHandler := handlers.NewDeleteCardHandler(softDelete, o11y)
	invoiceForHandler := handlers.NewInvoiceForHandler(invoiceFor, o11y)
	bestPurchaseDayHandler := handlers.NewBestPurchaseDayHandler(bestPurchaseDay, o11y)

	userRateLimit := ratelimit.NewRateLimitMiddleware(ctx, ratelimit.RateLimitConfig{
		PerMinute: cfg.AuthRateLimit.PerUserPerMin,
		Burst:     cfg.AuthRateLimit.PerUserBurst,
		Extractor: ratelimit.ByUserID,
		Scope:     "user",
	}, o11y)

	router := httpserver.NewCardRouter(createHandler, listHandler, getHandler, updateHandler, deleteHandler, invoiceForHandler, bestPurchaseDayHandler, idemStorage, o11y, gatewayAuth, userRateLimit)

	var eventHandlers []EventHandlerRegistration

	invoiceDueAlertsJob, invoiceDueEventHandlers := newInvoiceDueArtifacts(
		cfg,
		o11y,
		db,
		factory,
		loc,
		channelGateway,
		channelResolver,
	)
	eventHandlers = append(eventHandlers, invoiceDueEventHandlers...)

	return CardModule{
		RepositoryFactory:       factory,
		CardRouter:              router,
		CardLookup:              getCardForUser,
		ListCardsUC:             listCards,
		CountCardsUC:            countCards,
		CreateCardUC:            createCard,
		GetCardUC:               getCard,
		ResolveCardByNicknameUC: resolveCardByNickname,
		UpdateCardUC:            updateCard,
		SoftDeleteCardUC:        softDelete,
		InvoiceForUC:            invoiceFor,
		BestPurchaseDayUC:       bestPurchaseDay,
		IsBankRecognizedUC:      isBankRecognized,
		InvoiceDueAlertsJob:     invoiceDueAlertsJob,
		EventHandlers:           eventHandlers,
	}, nil
}

func newInvoiceDueArtifacts(
	cfg *configs.Config,
	o11y observability.Observability,
	db *sqlx.DB,
	factory interfaces.RepositoryFactory,
	loc *time.Location,
	channelGateway notification.ChannelGateway,
	channelResolver interfaces.UserChannelResolver,
) (worker.Job, []EventHandlerRegistration) {
	if channelGateway == nil || channelResolver == nil {
		return nil, nil
	}

	outboxFactory := outbox.NewRepositoryFactory(o11y)
	invoiceDuePublisher := cardproducers.NewInvoiceDuePublisher(outboxFactory, cfg.OutboxConfig, id.NewUUIDGenerator(), o11y)
	evaluateUoW := uow.NewUnitOfWork(db)
	evaluateInvoiceDue := usecases.NewEvaluateInvoiceDueAlerts(
		factory,
		invoiceDuePublisher,
		evaluateUoW,
		loc,
		cfg.CardConfig.InvoiceDueWindowDays,
		cfg.CardConfig.InvoiceDueScanLimit,
		o11y,
	)
	alertSentRepo := factory.InvoiceDueAlertSentRepository(db)
	notifyInvoiceDue := usecases.NewNotifyInvoiceDue(alertSentRepo, channelResolver, channelGateway, loc, o11y)
	invoiceDueNotifier := cardconsumers.NewInvoiceDueNotifier(notifyInvoiceDue, o11y)
	eventHandlers := []EventHandlerRegistration{
		{
			EventType: "card.invoice_due.v1",
			Handler:   invoiceDueNotifier,
		},
	}

	if !cfg.CardConfig.InvoiceDueAlertsEnabled {
		return nil, eventHandlers
	}

	return cardjobs.NewInvoiceDueAlertsJob(evaluateInvoiceDue, cfg.CardConfig), eventHandlers
}
