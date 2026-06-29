package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	agentapplication "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/agents"
	agentscorers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/scorers"
	agenttools "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/infrastructure/messaging/database/consumers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/infrastructure/weather"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	agentpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent/infrastructure/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory/infrastructure/indexer"
	memorypostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory/infrastructure/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/scorer"
	scorerpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/scorer/infrastructure/postgres"
	wadispatcher "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dispatcher"
	wapayload "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/payload"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/worker"
)

const EventTypeWhatsAppInbound = "agents.whatsapp.inbound.v1"

var errLLMAPIKeyRequired = errors.New("agents.module: llm api_key is required")

type whatsAppGateway interface {
	SendTextMessage(ctx context.Context, toE164, text string) error
}

type EventHandlerRegistration struct {
	EventType string
	Handler   events.Handler
}

type Module struct {
	HandleInbound      *usecases.HandleInbound
	WhatsAppAgentRoute func(ctx context.Context, msg wapayload.Message) wadispatcher.RouteOutcome
	EventHandlers      []EventHandlerRegistration
	Jobs               []worker.Job
	scorerRunner       scorer.ScorerRunner
}

func (m Module) Shutdown(ctx context.Context) {
	if m.scorerRunner != nil {
		m.scorerRunner.Shutdown(ctx)
	}
}

type LLMConfig struct {
	Model       string
	EmbedModel  string
	APIKey      string
	BaseURL     string
	MaxTokens   int
	Temperature float64
}

type Deps struct {
	DB              database.DBTX
	O11y            observability.Observability
	OutboxPublisher outbox.Publisher
	LLM             LLMConfig
	WeatherClient   weather.Client
	WhatsAppGateway whatsAppGateway
}

type whatsAppInboundPayload struct {
	UserID    string `json:"user_id"`
	Peer      string `json:"peer"`
	Text      string `json:"text"`
	MessageID string `json:"message_id"`
}

func NewModule(deps Deps) (Module, error) {
	if deps.DB == nil {
		return Module{}, fmt.Errorf("agents.module: db is required")
	}
	if deps.O11y == nil {
		return Module{}, fmt.Errorf("agents.module: o11y is required")
	}
	if deps.LLM.APIKey == "" {
		return Module{}, errLLMAPIKeyRequired
	}

	httpClient, err := httpclient.NewClient(deps.O11y,
		httpclient.WithBaseURL(deps.LLM.BaseURL),
		httpclient.WithTarget("openrouter"),
	)
	if err != nil {
		return Module{}, fmt.Errorf("agents.module: http client: %w", err)
	}

	provider := llm.NewOpenRouterProvider(httpClient, llm.Config{
		Model:       deps.LLM.Model,
		EmbedModel:  deps.LLM.EmbedModel,
		APIKey:      deps.LLM.APIKey,
		BaseURL:     deps.LLM.BaseURL,
		MaxTokens:   deps.LLM.MaxTokens,
		Temperature: deps.LLM.Temperature,
	}, deps.O11y)

	scorerResultStore := scorerpostgres.NewResultStore(deps.DB)
	scorerEntries := agentscorers.BuildWeatherScorers(provider)
	scorerRunner := scorer.NewScorerRunner(scorerEntries, scorerResultStore, deps.O11y)
	scoringHooks := agentapplication.NewScoringHooks(scorerRunner)

	weatherTool := agenttools.BuildWeatherTool(deps.WeatherClient)
	weatherAgent := agentapplication.BuildWeatherAgent(provider, weatherTool, scoringHooks, deps.O11y)

	registry := agent.NewAgentRegistry()
	registry.Register(weatherAgent)

	threadGateway := memorypostgres.NewThreadRepository(deps.DB, deps.O11y)
	rawMessageStore := memorypostgres.NewMessageRepository(deps.DB, deps.O11y)
	workingMem := memorypostgres.NewWorkingMemoryRepository(deps.DB, deps.O11y)
	semanticRecall := memorypostgres.NewEmbeddingRepository(deps.DB, deps.O11y)

	messageStore := rawMessageStore
	var eventHandlers []EventHandlerRegistration
	if deps.OutboxPublisher != nil {
		indexPublisher := indexer.NewOutboxMessageIndexPublisher(deps.OutboxPublisher)
		messageStore = memory.NewPublishingMessageStore(rawMessageStore, indexPublisher, deps.LLM.EmbedModel, deps.O11y)
		indexHandler := indexer.NewEmbeddingIndexHandler(provider, semanticRecall, deps.O11y)
		eventHandlers = append(eventHandlers, EventHandlerRegistration{
			EventType: memory.EventTypeEmbeddingIndex,
			Handler:   indexHandler,
		})
	}

	runStore := agentpostgres.NewRunStore(deps.DB)

	runtime := agent.NewAgentRuntime(registry, threadGateway, messageStore, workingMem, runStore, deps.O11y)

	handleInbound := usecases.NewHandleInbound(runtime, deps.O11y)

	var whatsAppRoute func(ctx context.Context, msg wapayload.Message) wadispatcher.RouteOutcome
	if deps.OutboxPublisher != nil && deps.WhatsAppGateway != nil {
		inboundConsumer := consumers.NewWhatsAppInboundConsumer(handleInbound, deps.WhatsAppGateway, deps.O11y)
		eventHandlers = append(eventHandlers, EventHandlerRegistration{
			EventType: EventTypeWhatsAppInbound,
			Handler:   inboundConsumer,
		})
		whatsAppRoute = buildWhatsAppAgentRoute(deps.OutboxPublisher, deps.O11y)
	}

	return Module{
		HandleInbound:      handleInbound,
		WhatsAppAgentRoute: whatsAppRoute,
		EventHandlers:      eventHandlers,
		scorerRunner:       scorerRunner,
	}, nil
}

func buildWhatsAppAgentRoute(publisher outbox.Publisher, o11y observability.Observability) func(ctx context.Context, msg wapayload.Message) wadispatcher.RouteOutcome {
	routeTotal := o11y.Metrics().Counter(
		"agents_whatsapp_route_total",
		"Total de mensagens roteadas para o agente via WhatsApp",
		"1",
	)
	return func(ctx context.Context, msg wapayload.Message) wadispatcher.RouteOutcome {
		ctx, span := o11y.Tracer().Start(ctx, "agents.route.whatsapp_inbound")
		defer span.End()

		principal, ok := auth.FromContext(ctx)
		if !ok {
			o11y.Logger().Error(ctx, "agents.route.whatsapp_inbound: principal ausente no contexto")
			routeTotal.Add(ctx, 1, observability.String("outcome", "no_principal"))
			return wadispatcher.OutcomeInvalid
		}

		p := whatsAppInboundPayload{
			UserID:    principal.UserID.String(),
			Peer:      msg.From,
			Text:      msg.Text,
			MessageID: msg.WAMID,
		}

		raw, err := json.Marshal(p)
		if err != nil {
			o11y.Logger().Error(ctx, "agents.route.whatsapp_inbound: marshal payload", observability.Error(err))
			routeTotal.Add(ctx, 1, observability.String("outcome", "error"))
			return wadispatcher.OutcomeInvalid
		}

		eventID, err := uuid.NewV7()
		if err != nil {
			o11y.Logger().Error(ctx, "agents.route.whatsapp_inbound: gerar event id", observability.Error(err))
			routeTotal.Add(ctx, 1, observability.String("outcome", "error"))
			return wadispatcher.OutcomeInvalid
		}

		evt, err := outbox.NewEvent(outbox.EventInput{
			ID:              eventID.String(),
			Type:            EventTypeWhatsAppInbound,
			AggregateType:   "whatsapp.message",
			AggregateID:     msg.WAMID,
			AggregateUserID: principal.UserID.String(),
			Payload:         raw,
			OccurredAt:      time.Now().UTC(),
		})
		if err != nil {
			o11y.Logger().Error(ctx, "agents.route.whatsapp_inbound: criar evento", observability.Error(err))
			routeTotal.Add(ctx, 1, observability.String("outcome", "error"))
			return wadispatcher.OutcomeInvalid
		}

		if err := publisher.Publish(ctx, evt); err != nil {
			o11y.Logger().Error(ctx, "agents.route.whatsapp_inbound: publicar evento", observability.Error(err))
			span.RecordError(err)
			routeTotal.Add(ctx, 1, observability.String("outcome", "error"))
			return wadispatcher.OutcomeInvalid
		}

		routeTotal.Add(ctx, 1, observability.String("outcome", "routed"))
		return wadispatcher.OutcomeAgent
	}
}
