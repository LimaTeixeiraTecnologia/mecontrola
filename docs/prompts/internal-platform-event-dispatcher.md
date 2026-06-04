# Prompt original

Quero implementar um event dispatcher com base nos exemplos abaixo.

E inegociavel e mandatorio:
1. utilizar `go-implementation`;
2. carregar os exemplos da skill;
3. focar em robustez, eficiencia, production-ready e production-proof;
4. evitar falso positivo arquitetural;
5. entregar `0 comentarios` no codigo final.

Exemplos de referencia obrigatoria:

```go
package events

import (
    "context"
)

type Event interface {
    GetEventType() string
    GetPayload() any
}

type EventDispatcher interface {
    Register(eventType string, handler EventHandler) error
    Dispatch(ctx context.Context, event Event) error
    Remove(eventType string, handler EventHandler) error
    Has(eventType string, handler EventHandler) bool
    Clear()
}

type EventHandler interface {
    Handle(ctx context.Context, event Event) error
}
```

```go
package events

import (
    "context"
    "errors"
    "slices"
    "sync"
)

var (
    ErrHandlerAlreadyRegistered = errors.New("handler already registered")
    ErrEventNil                 = errors.New("event cannot be nil")
    ErrHandlerNil               = errors.New("handler cannot be nil")
    ErrEventTypeEmpty           = errors.New("event type cannot be empty")
)

type eventDispatcher struct {
    mu       sync.RWMutex
    handlers map[string][]EventHandler
}

type DispatcherOption func(*eventDispatcher)

func WithCapacity(capacity int) DispatcherOption {
    return func(ed *eventDispatcher) {
        ed.handlers = make(map[string][]EventHandler, capacity)
    }
}

func NewEventDispatcher(opts ...DispatcherOption) EventDispatcher {
    ed := &eventDispatcher{
        handlers: make(map[string][]EventHandler),
    }

    for _, opt := range opts {
        opt(ed)
    }

    return ed
}

func (ed *eventDispatcher) Dispatch(ctx context.Context, event Event) error {
    if event == nil {
        return ErrEventNil
    }

    eventType := event.GetEventType()
    if eventType == "" {
        return ErrEventTypeEmpty
    }

    ed.mu.RLock()
    handlers, ok := ed.handlers[eventType]
    if !ok {
        ed.mu.RUnlock()
        return nil
    }

    handlersCopy := make([]EventHandler, len(handlers))
    copy(handlersCopy, handlers)
    ed.mu.RUnlock()

    for _, handler := range handlersCopy {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        if err := handler.Handle(ctx, event); err != nil {
            return err
        }
    }
    return nil
}

func (ed *eventDispatcher) Register(eventType string, handler EventHandler) error {
    if eventType == "" {
        return ErrEventTypeEmpty
    }
    if handler == nil {
        return ErrHandlerNil
    }

    ed.mu.Lock()
    defer ed.mu.Unlock()

    if slices.Contains(ed.handlers[eventType], handler) {
        return ErrHandlerAlreadyRegistered
    }

    ed.handlers[eventType] = append(ed.handlers[eventType], handler)
    return nil
}

func (ed *eventDispatcher) Has(eventType string, handler EventHandler) bool {
    if eventType == "" || handler == nil {
        return false
    }

    ed.mu.RLock()
    defer ed.mu.RUnlock()

    handlers, ok := ed.handlers[eventType]
    if !ok {
        return false
    }

    return slices.Contains(handlers, handler)
}

func (ed *eventDispatcher) Remove(eventType string, handler EventHandler) error {
    if eventType == "" || handler == nil {
        return nil
    }

    ed.mu.Lock()
    defer ed.mu.Unlock()

    handlers, ok := ed.handlers[eventType]
    if !ok {
        return nil
    }

    found := slices.Contains(handlers, handler)
    if !found {
        return nil
    }

    newHandlers := make([]EventHandler, 0, len(handlers)-1)
    removed := false
    for _, h := range handlers {
        if h == handler && !removed {
            removed = true
            continue
        }
        newHandlers = append(newHandlers, h)
    }

    if len(newHandlers) == 0 {
        delete(ed.handlers, eventType)
        return nil
    }

    ed.handlers[eventType] = newHandlers
    return nil
}

func (ed *eventDispatcher) Clear() {
    ed.mu.Lock()
    defer ed.mu.Unlock()

    clear(ed.handlers)
}
```

Exemplo de uso para producer no `module.go` de cada modulo:

```go
func RegisterPublishEventHandler(ioc *bundle.Container, o11y observability.Observability) *job.PublishEventHandler {
    uow := unitOfWork.NewUnitOfWork(ioc.DB)
    repositoryFactory := repositories.NewRepositoryFactory()
    brokerClient := kafka.NewKafkaClient(ioc.Config.KafkaConfig.Brokers[0], o11y)
    publishEventUseCase := usecase.NewPublishEventUseCase(uow, ioc.Config, o11y, brokerClient, repositoryFactory)
    return job.NewPublishEventHandler(o11y, publishEventUseCase)
}
```

# Prompt enriquecido

```text
Quero que voce implemente um event dispatcher em Go para este repositorio, com foco em uso compartilhado por modulos, forte aderencia arquitetural ao monolito modular atual e integracao coerente com o desenho ja existente em `internal/platform/worker`.

Este pedido e INEGOCIAVEL:
1. use obrigatoriamente `AGENTS.md` como fonte canonica;
2. carregue obrigatoriamente `.github/skills/agent-governance/SKILL.md`;
3. carregue obrigatoriamente `.github/skills/go-implementation/SKILL.md`;
4. carregue obrigatoriamente os exemplos da skill `go-implementation` relevantes para esta tarefa;
5. respeite obrigatoriamente a versao declarada em `go.mod`;
6. trate o estado atual do working tree como fonte da verdade;
7. comece a analise obrigatoriamente por `cmd/server/server.go` e/ou `cmd/worker/worker.go`;
8. nao use `internal/platform/runtime` como ponto de partida ou referencia de composicao;
9. entregue codigo final com `0 comentarios`;
10. mantenha foco em robustez, eficiencia, production-ready, production-proof e sem falso positivo arquitetural.

## Carga obrigatoria antes de qualquer edicao

Base:
1. `AGENTS.md`
2. `.github/skills/agent-governance/SKILL.md`
3. `.github/skills/go-implementation/SKILL.md`
4. `go.mod`

Referencias da skill Go que devem ser carregadas com economia de contexto, no maximo 4 por vez:
1. `references/architecture.md`
2. `references/interfaces.md`
3. `references/concurrency.md`
4. `references/patterns-behavioral.md`
5. `references/messaging.md`
6. `references/testing.md`
7. `references/examples-domain-flow.md`
8. `references/examples-testing.md`
9. `references/examples-infrastructure.md`

Se precisar de mais de 4 referencias simultaneas, priorize as 3 mais criticas para a etapa atual e registre as demais como contexto nao carregado.

## Contexto real do repositorio que deve orientar a implementacao

- O projeto e um monolito modular em Go.
- `go.mod` declara Go `1.26.2`.
- `cmd/server/server.go` e `cmd/worker/worker.go` fazem o bootstrap principal da aplicacao.
- Ambos os entrypoints usam `internal/platform/database`, `internal/platform/observability` e `internal/platform/worker`.
- O worker compartilhado ja possui contratos e implementacoes relevantes em:
  - `internal/platform/worker/manager.go`
  - `internal/platform/worker/job/adapter.go`
  - `internal/platform/worker/consumer/types.go`
  - `internal/platform/worker/consumer/registry.go`
  - `internal/platform/worker/consumer/adapter.go`
  - `internal/platform/worker/consumer/database/adapter.go`
- O codigo atual ja tem um `consumer.Registry` com `Register(reg Registration) error` e `Dispatch(ctx, eventType, params, body) error`.
- O codigo atual ja tem `job.Adapter`, `consumer.Adapter` e `worker.Manager`, portanto a implementacao do event dispatcher NAO pode duplicar abstrações ja existentes sem justificativa tecnica objetiva.
- No working tree atual, `cmd/server/server.go` e `cmd/worker/worker.go` ainda importam `internal/billing` e `internal/identity`, mas nao existem `internal/**/module.go` correspondentes no estado local atual. Considere isso drift do working tree e nao invente contexto ausente.
- O estado atual do working tree prevalece sobre exemplos historicos, documentacao anterior e solucoes especulativas.

## Objetivo principal

Implementar um event dispatcher com API inspirada nos exemplos fornecidos, adaptado ao desenho real do repositorio, para permitir registro e despacho seguro de handlers por `eventType`, com possibilidade de uso por producers/registrations de modulo e sem romper fronteiras arquiteturais.

## API de referencia obrigatoria

Use estes contratos como referencia obrigatoria de comportamento, adaptando nomes, package e detalhes apenas quando houver conflito objetivo com o codebase:

```go
type Event interface {
    GetEventType() string
    GetPayload() any
}

type EventDispatcher interface {
    Register(eventType string, handler EventHandler) error
    Dispatch(ctx context.Context, event Event) error
    Remove(eventType string, handler EventHandler) error
    Has(eventType string, handler EventHandler) bool
    Clear()
}

type EventHandler interface {
    Handle(ctx context.Context, event Event) error
}
```

## Decisoes arquiteturais obrigatorias

1. Antes de criar um novo pacote, avalie se o comportamento desejado deve:
   - reutilizar o `consumer.Registry` existente;
   - extrair comportamento compartilhado sem quebrar `internal/platform/worker/consumer`;
   - ou criar uma nova capacidade compartilhada, preferencialmente em `internal/platform/events`, apenas se houver diferenca semantica real entre `dispatcher de eventos de dominio/aplicacao` e `registry de consumer`.
2. E proibido criar uma abstracao paralela apenas com nomes diferentes se ela resolver exatamente o mesmo problema de `consumer.Registry`.
3. Se optar por um novo pacote, deixe explicito por que `consumer.Registry` nao atende integralmente o contrato do event dispatcher.
4. Preserve o fluxo arquitetural `infrastructure -> application -> domain`.
5. Interfaces devem ficar no consumidor, nao no produtor.
6. Nao vaze detalhes de broker, HTTP, banco, scheduler ou observability para dentro do contrato do dispatcher.
7. Nao use estado global, `init()`, `panic` ou fallback silencioso.
8. Use `context.Context` em toda fronteira de IO e respeite cancelamento durante o dispatch.
9. Se houver necessidade de agregacao de erros, use `errors.Join`; para wrapping contextual, use `fmt.Errorf("ctx: %w", err)`.
10. Nao introduza comentarios no codigo final.

## Requisitos funcionais

1. O dispatcher deve permitir:
   - registrar handler por `eventType`;
   - despachar evento por `eventType`;
   - remover handler registrado;
   - verificar se handler ja esta registrado;
   - limpar todos os registros.
2. O dispatcher deve validar:
   - `event == nil`;
   - `handler == nil`;
   - `eventType == ""`.
3. O dispatcher deve impedir registro duplicado do mesmo handler para o mesmo `eventType`.
4. O dispatcher deve ser seguro para concorrencia.
5. O dispatch deve trabalhar com snapshot dos handlers para evitar corrida entre leitura e mutacao.
6. O dispatch deve respeitar cancelamento do `context.Context` antes de executar cada handler.
7. O dispatch deve preservar a ordem de execucao dos handlers registrados, salvo se voce encontrar no codebase um requisito contrario claramente existente.
8. Se nao houver handlers para um `eventType`, o comportamento deve ser explicito e coerente com o repositorio; nao invente erro onde o design atual espera no-op, nem silencie erro onde o design atual exige falha.

## Requisitos de integracao com o repositorio

1. Comece a analise por:
   - `cmd/server/server.go`
   - `cmd/worker/worker.go`
2. Inspecione obrigatoriamente antes de editar:
   - `internal/platform/worker/manager.go`
   - `internal/platform/worker/job/adapter.go`
   - `internal/platform/worker/consumer/types.go`
   - `internal/platform/worker/consumer/registry.go`
   - `internal/platform/worker/consumer/adapter.go`
   - `internal/platform/worker/consumer/database/adapter.go`
3. Verifique se o dispatcher deve conversar com:
   - registrations/handlers de consumer ja existentes;
   - producers de modulo;
   - bootstrap de modulo equivalente a `module.go`, quando esse arquivo existir;
   - ou outro ponto de composicao existente no modulo real.
4. O exemplo abaixo deve ser tratado como referencia de composicao para producer no modulo, sem copiar cegamente e sem inventar `module.go` se o modulo real usar outro entrypoint:

```go
func RegisterPublishEventHandler(ioc *bundle.Container, o11y observability.Observability) *job.PublishEventHandler {
    uow := unitOfWork.NewUnitOfWork(ioc.DB)
    repositoryFactory := repositories.NewRepositoryFactory()
    brokerClient := kafka.NewKafkaClient(ioc.Config.KafkaConfig.Brokers[0], o11y)
    publishEventUseCase := usecase.NewPublishEventUseCase(uow, ioc.Config, o11y, brokerClient, repositoryFactory)
    return job.NewPublishEventHandler(o11y, publishEventUseCase)
}
```

5. Se o modulo real nao tiver `module.go`, adapte o registro ao ponto de composicao existente mais proximo, sem criar camadas artificiais.
6. Preserve o uso do identificador `o11y` para observabilidade; nao renomeie para `provider` quando isso virar variavel de modulo ou dependencia de observabilidade.

## Requisitos nao funcionais

1. Production-ready real:
   - sem race condition;
   - sem duplicacao desnecessaria de abstrações;
   - sem acoplamento indevido entre plataforma e modulo;
   - sem violacao de fronteira arquitetural;
   - sem falso positivo de "parece bom" quando o desenho conflita com o codebase.
2. Eficiencia:
   - lock minimo necessario;
   - copy defensivo apenas onde agrega seguranca real;
   - sem alocacoes gratuitas ou paths de erro desnecessariamente caros.
3. Robustez:
   - erros sentinela quando fizer sentido;
   - mensagens de erro curtas, contextuais e consistentes com o repositorio;
   - cobertura de edge cases reais;
   - sem comportamento ambiguo para duplicidade, remocao inexistente ou dispatch sem handlers.
4. Testes:
   - adicionar ou ajustar testes proporcionais ao impacto;
   - incluir cenarios de registro duplicado, dispatch com sucesso, handler retornando erro, cancelamento de contexto, remocao, limpeza e concorrencia basica;
   - usar os exemplos de teste da skill `go-implementation` como referencia.

## Proibicoes explicitas

- Nao implementar uma versao isolada do dispatcher sem primeiro comparar com `internal/platform/worker/consumer/registry.go`.
- Nao usar `var _ Interface = (*Type)(nil)`.
- Nao usar `interface{}`; use `any`.
- Nao usar comentarios no codigo final.
- Nao usar `_ = dependencia` para silenciar valor nao utilizado.
- Nao criar dependencias circulares entre `internal/platform` e modulos.
- Nao inventar arquivos, modulos ou registries que nao existam sem necessidade concreta.
- Nao assumir que `module.go` existe so porque o exemplo menciona isso.
- Nao alterar comportamento publico sem explicitar a mudanca.

## Criterios de aceitacao

1. Existe uma implementacao coerente de event dispatcher adaptada ao desenho real do repositorio.
2. A implementacao reutiliza ou extrai comportamento do stack atual de `internal/platform/worker/consumer` quando isso for arquiteturalmente correto, em vez de duplicar abstracoes.
3. Se houver novo pacote, a justificativa para ele e objetiva e tecnicamente defensavel.
4. O dispatcher cobre `Register`, `Dispatch`, `Remove`, `Has` e `Clear`.
5. O codigo e seguro para concorrencia e respeita `context.Context`.
6. O comportamento para duplicidade, handler nil, evento nil e `eventType` vazio e explicitamente tratado.
7. A integracao com producers/registrations de modulo respeita o ponto de composicao real do repositorio.
8. A implementacao segue obrigatoriamente `go-implementation`, seus exemplos e as regras R0-R7.
9. O codigo final possui `0 comentarios`.
10. A resposta final deve listar os arquivos alterados, explicar o desenho adotado e justificar qualquer decisao entre reutilizar `consumer.Registry` ou criar um dispatcher separado.

## Saida esperada do agente executor

1. Analise curta do acoplamento atual entre `cmd/server/server.go`, `cmd/worker/worker.go` e `internal/platform/worker`.
2. Implementacao completa e adaptada ao codebase real.
3. Testes proporcionais ao risco.
4. Resumo final em PT-BR, objetivo, sem floreio, deixando claro:
   - onde o dispatcher ficou;
   - como ele se relaciona com `consumer.Registry`;
   - como o registro em modulo foi resolvido;
   - e por que a solucao e production-ready de verdade.

Se houver conflito entre os exemplos fornecidos, o estado atual do repositorio, `AGENTS.md`, `agent-governance` e `go-implementation`, prevalecem `AGENTS.md`, o working tree atual e a restricao mais segura.
```

# Melhorias aplicadas

- Amarrei o prompt ao estado real do repositorio, exigindo analise inicial por `cmd/server/server.go` e `cmd/worker/worker.go`.
- Tornei obrigatoria a carga de `AGENTS.md`, `agent-governance`, `go-implementation`, `go.mod` e das referencias Go pertinentes com economia de contexto.
- Conectei o pedido ao stack ja existente em `internal/platform/worker`, especialmente `consumer.Registry`, `consumer.Adapter`, `consumer/database` e `worker.Manager`.
- Adicionei uma exigencia explicita para evitar duplicacao de abstracoes caso `consumer.Registry` ja resolva parte relevante do problema.
- Mantive o exemplo de registro de producer no modulo, mas eliminei a ambiguidade: se `module.go` nao existir, o agente deve usar o ponto real de composicao do modulo.
- Inclui criterios de aceitacao verificaveis para concorrencia, cancelamento de contexto, duplicidade, integracao e ausencia total de comentarios.
- Preservei o uso de `o11y` como identificador de observabilidade no prompt enriquecido.

# Variantes validas

## Variante recomendada

Implementar o dispatcher como evolucao ou extracao pequena a partir do comportamento ja existente em `internal/platform/worker/consumer/registry.go`, caso a analise mostre alta sobreposicao semantica.

## Variante aceitavel

Criar um pacote compartilhado separado, preferencialmente em `internal/platform/events`, apenas se a analise mostrar que o contrato `Event -> EventHandler` precisa coexistir com `consumer.Registry` sem misturar eventos de dominio/aplicacao com envelopes de consumo.
