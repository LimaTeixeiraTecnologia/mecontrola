# Prompt enriquecido

```text
Quero que voce implemente uma capacidade compartilhada em `internal/platform` para orquestracao de workers, cron jobs e consumers reutilizavel entre modulos do monolito, com foco inegociavel em robustez, eficiencia operacional, previsibilidade de lifecycle e readiness real de producao.

NAO EXISTE margem para desenho alternativo nestes pontos:
1. deve existir um `WorkerManager` como unico orquestrador central de lifecycle;
2. todo cron job deve obrigatoriamente ser registrado e inicializado via `JobAdapter`;
3. todo consumer deve obrigatoriamente ser registrado e inicializado via `ConsumerAdapter`;
4. jobs e consumers devem subir no startup do processo sob coordenacao explicita do `WorkerManager`;
5. jobs e consumers devem rodar em goroutines distintas, com cancelamento cooperativo, shutdown gracioso e espera coordenada;
6. qualquer desenho que inicialize cron jobs ou consumers por fora do `WorkerManager` deve ser considerado incorreto;
7. qualquer desenho que execute cron job sem `JobAdapter` ou consumer sem `ConsumerAdapter` deve ser considerado incorreto.

Este trabalho e INEGOCIAVEL: precisa ser robusto, eficiente, production-ready, production-proof e sem falso positivo arquitetural. Nao entregue um desenho "parecido"; entregue exatamente o contrato abaixo.

Tambem e obrigatorio, mandatorio e inegociavel:
1. usar `AGENTS.md` como fonte canonica do repositorio;
2. carregar `.github/skills/agent-governance/SKILL.md`;
3. carregar `.github/skills/go-implementation/SKILL.md`;
4. respeitar a versao declarada em `go.mod`;
5. manter todos os exemplos deste prompt SEM `uber/fx`, sem `fx.Lifecycle`, sem `dig` e sem framework de DI runtime;
6. preservar o fluxo arquitetural `handler -> usecase -> repositories e/ou client http`;
7. manter `internal/platform` desacoplado de `internal/<modulo>/...`;
8. nao colocar regra de negocio em `internal/platform`;
9. nao criar pacote global de clock em `internal/platform`;
10. nao adicionar comentarios no codigo final entregue.

## Contexto arquitetural obrigatorio

- O projeto e um monolito modular em Go.
- Capacidades tecnicas compartilhadas entre modulos devem viver em `internal/platform`.
- `internal/platform` nao pode importar codigo de modulos de negocio.
- Jobs concretos de modulo devem chamar use cases da camada `application`.
- Consumers concretos de modulo devem chamar use cases da camada `application`.
- O manager pode orquestrar lifecycle, mas nao pode executar regra de negocio diretamente.
- Se houver integracao HTTP em modulo de negocio, ela deve continuar em `internal/<modulo>/infrastructure/http/client`.
- `memory` NAO significa fila volatil em RAM; significa transporte local persistido usando o banco/outbox ja existente no sistema.
- E proibido reimplementar ou alterar internamente o outbox para acomodar esta estrutura; a integracao deve ocorrer por composicao.

## O que obrigatoriamente vai existir

### 1. WorkerManager

O `WorkerManager` deve ser o unico ponto de:
- registro final consolidado;
- validacao de startup;
- start;
- stop;
- graceful shutdown;
- coordenacao de goroutines;
- consolidacao de erros de lifecycle;
- espera de encerramento;
- observabilidade do lifecycle.

O `WorkerManager` deve receber explicitamente:
- uma colecao de jobs montada por `JobAdapter`;
- uma colecao de consumers montada por `ConsumerAdapter`;
- configuracao de lifecycle;
- dependencias tecnicas compartilhadas estritamente necessarias, como logger e clock local se houver necessidade local de infraestrutura.

O `WorkerManager` NAO pode:
- conhecer repositories concretos;
- conhecer use cases concretos;
- conhecer handlers HTTP;
- conhecer detalhes de modulo;
- instanciar sozinho cron jobs ou consumers fora dos adapters;
- esconder erro de startup/shutdown com fallback silencioso.

### 2. JobAdapter

Todo cron job deve obrigatoriamente passar por `JobAdapter`.

O `JobAdapter` deve:
- adaptar `name + schedule + execution fn` para o contrato padrao de job;
- carregar metadados operacionais minimos do job;
- permitir politica explicita de overlap;
- permitir cancelamento por `context.Context`;
- permanecer pequeno, reutilizavel e desacoplado de modulo.

E proibido:
- registrar cron job "na mao" direto no scheduler fora do adapter;
- deixar job concreto bypassar `JobAdapter`;
- acoplar `JobAdapter` a regras de negocio ou a configs de um modulo especifico.

### 3. ConsumerAdapter

Todo consumer deve obrigatoriamente passar por `ConsumerAdapter`.

O `ConsumerAdapter` deve:
- adaptar `name + technology + registry + source/runner` para um runner padrao de consumer;
- encapsular start/stop/lifecycle do consumer;
- padronizar o registro de handlers;
- padronizar observabilidade e tratamento de erro do loop de consumo;
- manter a regra de negocio desacoplada do broker.

O contrato do handler do consumer deve seguir obrigatoriamente:
`Handler(ctx context.Context, params map[string]string, body []byte) error`

O registro de handlers deve seguir obrigatoriamente o modelo:
`registry.Register(eventType, handler)`
ou equivalente semanticamente identico.

O `ConsumerAdapter` deve suportar um contrato unificado para:
- `memory`
- `rabbitmq`
- `kafka`
- `sqs`
- `azure service bus`

Para `memory`, trate explicitamente como:
- canal local persistido via banco/outbox;
- semantica de entrega local persistida;
- sem fila volatil em memoria substituindo persistencia;
- sem alterar a implementacao do outbox existente.

## Onde entra o consumer de forma agnostica

O ponto agnostico NAO deve ser o client concreto de Kafka, RabbitMQ, PubSub ou banco-fila. O ponto agnostico deve ser o contrato de consumo e o registro dos handlers.

Portanto, a separacao obrigatoria deve ser:

### Camada compartilhada em `internal/platform`

Em `internal/platform`, devem existir apenas contratos e coordenacao tecnica compartilhada:
- `ConsumerAdapter`;
- `Registry`;
- `Registration`;
- `Runner` ou `Source`;
- normalizacao de envelope/mensagem;
- lifecycle compartilhado de consumer;
- binding entre transporte e registry;
- adapters por tecnologia.

### Camada do modulo em `internal/<modulo>/infrastructure`

No modulo, devem existir:
- handlers concretos;
- funcao de registro dos handlers do modulo;
- construcao do runner/source concreto para cada tecnologia;
- mapeamento de topicos/filas/subscriptions do modulo.

O modulo NAO deve registrar handlers diretamente no client concreto de Kafka/Rabbit/PubSub. O modulo deve registrar handlers apenas em um `Registry` agnostico. O adapter da tecnologia e quem consome esse `Registry` e conecta o transporte real ao mesmo conjunto de handlers.

## Onde entram os registros dos handlers

Os registros dos handlers devem viver no modulo, e nao no `internal/platform`.

Estrutura esperada no modulo:

```text
internal/<modulo>/
  infrastructure/
    messaging/
      consumers/
        handlers/
          settlement_completed.go
          create_crypto_reward.go
          pay_crypto_reward.go
        registrations.go
        bindings.go
        kafka/
          source.go
        rabbitmq/
          source.go
        memory/
          source.go
```

Responsabilidade de cada arquivo:
- `handlers/`: handlers concretos do modulo, um por caso de processamento;
- `registrations.go`: registra os handlers uma unica vez em `consumer.Registry`;
- `bindings.go`: descreve o mapeamento entre `eventType` e transporte/topico/fila/subscription quando necessario;
- `kafka/source.go`, `rabbitmq/source.go`, `memory/source.go`: criam a fonte concreta daquela tecnologia para o mesmo registry.

## Regra obrigatoria para reaproveitamento multi-tecnologia

Os mesmos handlers do modulo devem poder ser usados em Kafka, RabbitMQ, PubSub e no canal local persistido (`memory`) sem alteracao do codigo dos handlers.

Isso exige obrigatoriamente:
1. handler desacoplado do broker;
2. assinatura unica de handler;
3. registry unico e agnostico;
4. envelope normalizado pelo adapter da tecnologia;
5. metadata entregue em `params map[string]string`;
6. body entregue como `[]byte`;
7. event type resolvido pelo transporte e despachado pelo registry.

Em outras palavras:
- o modulo registra handlers uma unica vez;
- cada tecnologia apenas traduz sua mensagem para o contrato canonico;
- o dispatch para os handlers continua identico.

## Arquitetura final obrigatoria para consumo agnostico

Este prompt deve adotar uma unica arquitetura valida:
- `Registry` agnostico central como fonte unica de handlers;
- handlers e registrations no modulo;
- bindings de transporte separados no modulo;
- `ConsumerAdapter` recebendo `registry + source/runner + binding metadata`;
- `WorkerManager` orquestrando os adapters sem conhecer detalhes do broker;
- inicio obrigatorio por `memory` usando banco/outbox;
- evolucao prevista para RabbitMQ e Kafka sem alterar handlers nem registry.

Fluxo obrigatorio:
1. modulo constroi um unico `consumer.Registry`;
2. modulo registra todos os handlers uma unica vez;
3. modulo declara bindings por tecnologia em arquivo separado;
4. `MemorySource` le do banco/outbox, resolve `eventType` e despacha via `registry.Dispatch(...)`;
5. quando RabbitMQ ou Kafka entrarem, cada novo `Source` reutiliza os mesmos bindings e o mesmo registry;
6. o mesmo conjunto de handlers continua funcionando sem alteracao do codigo de negocio.

Razoes obrigatorias para essa escolha:
- melhor organizacao entre contrato funcional e topologia de transporte;
- menor risco de drift entre banco/outbox, RabbitMQ e Kafka;
- melhor evolucao incremental saindo de banco/outbox para broker real;
- bootstrap mais previsivel;
- maior testabilidade de dispatch, bindings e lifecycle;
- menor acoplamento entre regra de negocio e tecnologia de mensageria.

Consequencia arquitetural obrigatoria:
- o sistema deve nascer preparado para coexistencia de `memory`, RabbitMQ e Kafka, ainda que inicialmente so `memory` seja ligado no bootstrap;
- adicionar uma nova tecnologia deve significar apenas criar `source/adapter` e bindings, nunca reescrever handlers.

## Contrato canonico de registro

O registro agnostico deve ser representado semanticamente assim:

```go
type Handler interface {
    Handler(ctx context.Context, params map[string]string, body []byte) error
}

type Registration struct {
    Name      string
    EventType string
    Handler   Handler
}

type Registry interface {
    Register(reg Registration) error
    Dispatch(ctx context.Context, eventType string, params map[string]string, body []byte) error
}
```

O adapter concreto de tecnologia deve apenas:
- receber a mensagem do broker;
- extrair `eventType`;
- montar `params`;
- extrair `body`;
- chamar `registry.Dispatch(...)`.

## Exemplo completo final e obrigatorio com base no caso de settlement

Seu exemplo atual de `SettlementConsumer` deve ser convertido conceitualmente para o modelo abaixo, considerando o cenario real:
- hoje o publish ocorre no banco/outbox;
- um consumer/cron reader le esse canal persistido;
- o `eventType` define qual handler sera chamado;
- no futuro RabbitMQ e Kafka devem reutilizar exatamente os mesmos handlers.

### 1. Handlers continuam no modulo

```go
package consumers

import (
    "github.com/mercadobitcoin/pipelines-cards/apps/settlement/internal/application/dto"
    "github.com/mercadobitcoin/pipelines-cards/apps/settlement/internal/application/services"
    "github.com/mercadobitcoin/pipelines-cards/apps/settlement/internal/domain/ports"
    "github.com/mercadobitcoin/pipelines-cards/apps/settlement/internal/domain/vos"
    handlers "github.com/mercadobitcoin/pipelines-cards/apps/settlement/internal/infrastructure/kafka/consumer"
    lake "github.com/mercadobitcoin/pipelines-cards/pkg/clients/lake"
)

type SettlementHandlers struct {
    SettlementCompleted            consumer.Handler
    CreateCryptoReward             consumer.Handler
    PayCryptoReward                consumer.Handler
    ApplyCryptoRewardRefund        consumer.Handler
    CardOperationAdjustment        consumer.Handler
    AdjustmentReconciliation       consumer.Handler
    CryptoRewardCreated            consumer.Handler
    CryptoRewardRefundCreated      consumer.Handler
}

func NewSettlementHandlers(
    lake lake.LakeClient,
    settlementService services.SettlementService,
    payCryptoRewardService services.PayCryptoRewardService,
    createCryptoRewardService services.CreateCryptoRewardService,
    applyCryptoRewardRefundService services.ApplyCryptoRewardRefundService,
    cardOperationAdjustmentService services.CardOperationAdjustmentService,
    adjustmentReconciliationService services.AdjustmentReconciliationService,
    presentmentFilesRepository ports.PresentmentFilesRepository,
) SettlementHandlers {
    return SettlementHandlers{
        SettlementCompleted:       handlers.NewSettlementCompletedHandler(settlementService),
        CreateCryptoReward:        handlers.NewCreateCryptoRewardHandler(createCryptoRewardService),
        PayCryptoReward:           handlers.NewPayCryptoRewardHandler(payCryptoRewardService),
        ApplyCryptoRewardRefund:   handlers.NewApplyCryptoRewardRefundHandler(applyCryptoRewardRefundService),
        CardOperationAdjustment:   handlers.NewCardOperationAdjustmentHandler(cardOperationAdjustmentService),
        AdjustmentReconciliation:  handlers.NewAdjustmentReconciliationHandler(adjustmentReconciliationService),
        CryptoRewardCreated:       handlers.NewCryptoRewardCreatedHandler(lake, presentmentFilesRepository),
        CryptoRewardRefundCreated: handlers.NewCryptoRewardRefundCreatedHandler(lake, presentmentFilesRepository),
    }
}
```

### 2. Registro agnostico dos handlers no modulo

```go
package consumers

import (
    "github.com/mercadobitcoin/pipelines-cards/apps/settlement/internal/application/dto"
    "github.com/mercadobitcoin/pipelines-cards/apps/settlement/internal/domain/vos"
    "internal/platform/worker/consumer"
)

func RegisterSettlementHandlers(registry consumer.Registry, handlers SettlementHandlers) error {
    registrations := []consumer.Registration{
        {Name: "settlement-completed", EventType: dto.EventSettlementCompleted.String(), Handler: handlers.SettlementCompleted},
        {Name: "create-crypto-reward", EventType: vos.OperationPresented.String(), Handler: handlers.CreateCryptoReward},
        {Name: "pay-crypto-reward", EventType: vos.CryptoRewardsWaiting.String(), Handler: handlers.PayCryptoReward},
        {Name: "apply-crypto-reward-refund", EventType: vos.OperationRefund.String(), Handler: handlers.ApplyCryptoRewardRefund},
        {Name: "card-operation-adjustment", EventType: vos.OperationAdjustment.String(), Handler: handlers.CardOperationAdjustment},
        {Name: "adjustment-reconciliation", EventType: vos.PresentmentImportCompleted.String(), Handler: handlers.AdjustmentReconciliation},
        {Name: "crypto-reward-created", EventType: vos.CryptoRewardCreated.String(), Handler: handlers.CryptoRewardCreated},
        {Name: "crypto-reward-refund-created", EventType: vos.CryptoRewardRefundCreated.String(), Handler: handlers.CryptoRewardRefundCreated},
    }

    for _, registration := range registrations {
        if err := registry.Register(registration); err != nil {
            return err
        }
    }

    return nil
}
```

### 3. Bindings obrigatorios por tecnologia

```go
package consumers

type Binding struct {
    EventType     string
    Channel       string
    Topic         string
    Queue         string
    Subscription  string
}

func SettlementMemoryBindings(channel string) []Binding {
    return []Binding{
        {EventType: dto.EventSettlementCompleted.String(), Channel: channel},
        {EventType: vos.OperationPresented.String(), Channel: channel},
        {EventType: vos.CryptoRewardsWaiting.String(), Channel: channel},
        {EventType: vos.OperationRefund.String(), Channel: channel},
        {EventType: vos.OperationAdjustment.String(), Channel: channel},
        {EventType: vos.PresentmentImportCompleted.String(), Channel: channel},
        {EventType: vos.CryptoRewardCreated.String(), Channel: channel},
        {EventType: vos.CryptoRewardRefundCreated.String(), Channel: channel},
    }
}

func SettlementKafkaBindings(topic string) []Binding {
    return []Binding{
        {EventType: dto.EventSettlementCompleted.String(), Topic: topic},
        {EventType: vos.OperationPresented.String(), Topic: topic},
        {EventType: vos.CryptoRewardsWaiting.String(), Topic: topic},
        {EventType: vos.OperationRefund.String(), Topic: topic},
        {EventType: vos.OperationAdjustment.String(), Topic: topic},
        {EventType: vos.PresentmentImportCompleted.String(), Topic: topic},
        {EventType: vos.CryptoRewardCreated.String(), Topic: topic},
        {EventType: vos.CryptoRewardRefundCreated.String(), Topic: topic},
    }
}

func SettlementRabbitBindings(queue string) []Binding {
    return []Binding{
        {EventType: dto.EventSettlementCompleted.String(), Queue: queue},
        {EventType: vos.OperationPresented.String(), Queue: queue},
        {EventType: vos.CryptoRewardsWaiting.String(), Queue: queue},
        {EventType: vos.OperationRefund.String(), Queue: queue},
        {EventType: vos.OperationAdjustment.String(), Queue: queue},
        {EventType: vos.PresentmentImportCompleted.String(), Queue: queue},
        {EventType: vos.CryptoRewardCreated.String(), Queue: queue},
        {EventType: vos.CryptoRewardRefundCreated.String(), Queue: queue},
    }
}
```

### 4. Sources concretos por tecnologia consumindo o mesmo registry

```go
package bootstrap

func NewSettlementMemoryConsumer(cfg *config.Config, registry consumer.Registry, logger *slog.Logger) worker.Consumer {
    source := settlementmemory.NewSource(
        cfg,
        logger,
        settlementconsumers.SettlementMemoryBindings(cfg.OutboxChannel),
    )

    return consumer.NewAdapter("settlement-memory", "memory", consumer.NewRunner(source, registry, logger))
}

func NewSettlementKafkaConsumer(cfg *config.Config, registry consumer.Registry, logger *slog.Logger) worker.Consumer {
    source := settlementkafka.NewSource(
        cfg,
        logger,
        settlementconsumers.SettlementKafkaBindings(cfg.SettlementTopic),
    )

    return consumer.NewAdapter("settlement-kafka", "kafka", consumer.NewRunner(source, registry, logger))
}

func NewSettlementRabbitConsumer(cfg *config.Config, registry consumer.Registry, logger *slog.Logger) worker.Consumer {
    source := settlementrabbit.NewSource(
        cfg,
        logger,
        settlementconsumers.SettlementRabbitBindings(cfg.SettlementQueue),
    )

    return consumer.NewAdapter("settlement-rabbitmq", "rabbitmq", consumer.NewRunner(source, registry, logger))
}
```

### 5. Bootstrap final obrigatorio: hoje com banco/outbox, pronto para RabbitMQ e Kafka

```go
package bootstrap

func NewSettlementWorkers(
    cfg *config.Config,
    logger *slog.Logger,
    lake lake.LakeClient,
    settlementService services.SettlementService,
    payCryptoRewardService services.PayCryptoRewardService,
    createCryptoRewardService services.CreateCryptoRewardService,
    applyCryptoRewardRefundService services.ApplyCryptoRewardRefundService,
    cardOperationAdjustmentService services.CardOperationAdjustmentService,
    adjustmentReconciliationService services.AdjustmentReconciliationService,
    presentmentFilesRepository ports.PresentmentFilesRepository,
) ([]worker.Consumer, error) {
    registry := consumer.NewRegistry()

    handlers := settlementconsumers.NewSettlementHandlers(
        lake,
        settlementService,
        payCryptoRewardService,
        createCryptoRewardService,
        applyCryptoRewardRefundService,
        cardOperationAdjustmentService,
        adjustmentReconciliationService,
        presentmentFilesRepository,
    )

    if err := settlementconsumers.RegisterSettlementHandlers(registry, handlers); err != nil {
        return nil, err
    }

    consumers := []worker.Consumer{NewSettlementMemoryConsumer(cfg, registry, logger)}

    if cfg.EnableRabbitMQ {
        consumers = append(consumers, NewSettlementRabbitConsumer(cfg, registry, logger))
    }

    if cfg.EnableKafka {
        consumers = append(consumers, NewSettlementKafkaConsumer(cfg, registry, logger))
    }

    return consumers, nil
}
```

### 6. Exemplo obrigatorio do source `memory` lendo banco/outbox e despachando por `eventType`

```go
package memory

func NewSource(cfg *config.Config, logger *slog.Logger, bindings []consumers.Binding) consumer.Source {
    return outboxsource.New(
        outboxsource.Config{
            Channel:         cfg.OutboxChannel,
            PollInterval:    cfg.OutboxPollInterval,
            BatchSize:       cfg.OutboxBatchSize,
            ShutdownTimeout: cfg.ShutdownTimeout,
        },
        logger,
        bindings,
    )
}
```

```go
package consumer

type Message struct {
    EventType string
    Params    map[string]string
    Body      []byte
}

type Source interface {
    Start(context.Context, func(context.Context, Message) error) error
    Stop(context.Context) error
}

func NewRunner(source Source, registry Registry, logger *slog.Logger) Runner {
    return runnerFunc{
        start: func(ctx context.Context) error {
            return source.Start(ctx, func(messageCtx context.Context, msg Message) error {
                return registry.Dispatch(messageCtx, msg.EventType, msg.Params, msg.Body)
            })
        },
        stop: func(ctx context.Context) error {
            return source.Stop(ctx)
        },
    }
}
```

### 7. Leitura arquitetural obrigatoria desse exemplo

```text
1. handlers concretos continuam no modulo;
2. o modulo registra handlers uma unica vez em um registry agnostico;
3. o canal inicial e `memory`, lendo do banco/outbox;
4. `eventType` e resolvido pelo source e despachado pelo registry;
5. RabbitMQ e Kafka entram depois reutilizando os mesmos handlers e o mesmo registry;
6. o WorkerManager sobe e para esses consumers sem conhecer nenhum detalhe do broker.
```

## Regras adicionais inegociaveis para envelope e dispatch

Para que a reutilizacao multi-tecnologia seja production-ready de verdade, o prompt deve exigir:
- contrato canonico de envelope;
- extracao deterministica de `eventType`;
- metadata padronizada em `params`;
- correlation ID e trace context quando existirem;
- erro explicito quando `eventType` nao estiver registrado;
- idempotencia dos handlers quando houver entrega at-least-once;
- o source `memory` deve ler de forma persistida e paginada do banco/outbox, nunca de estrutura volatil em RAM;
- ack/commit apenas apos sucesso do processamento;
- politica explicita de retry e DLQ no adapter/source da tecnologia, nunca no handler de negocio.

## Atualizacao obrigatoria da estrutura final esperada do projeto

O desenho final esperado deve contemplar tambem o modulo consumindo a plataforma assim:

```text
internal/
  platform/
    worker/
      manager.go
      config.go
      types.go
      job/
        adapter.go
      consumer/
        adapter.go
        registry.go
        registration.go
        runner.go
        envelope.go
        source.go
        memory/
          adapter.go
        kafka/
          adapter.go
        rabbitmq/
          adapter.go
        sqs/
          adapter.go
        azureservicebus/
          adapter.go
  settlement/
    infrastructure/
      messaging/
        consumers/
          handlers/
            settlement_completed.go
            create_crypto_reward.go
            pay_crypto_reward.go
            apply_crypto_reward_refund.go
            card_operation_adjustment.go
            adjustment_reconciliation.go
            crypto_reward_created.go
            crypto_reward_refund_created.go
          registrations.go
          bindings.go
          rabbitmq/
            source.go
          memory/
            source.go
          kafka/
            source.go
```

## Lifecycle obrigatorio

### Startup obrigatorio

No startup do processo worker, a sequencia obrigatoria deve ser clara e auditavel:

1. carregar configuracao;
2. instanciar logger e dependencias tecnicas;
3. construir use cases e adapters concretos dos modulos;
4. montar a lista de jobs exclusivamente via `JobAdapter`;
5. montar a lista de consumers exclusivamente via `ConsumerAdapter`;
6. instanciar um unico `WorkerManager` com jobs e consumers;
7. chamar `WorkerManager.Start(ctx)`;
8. o manager deve iniciar jobs e consumers em loops/goroutines independentes;
9. o processo deve permanecer vivo ate sinal de shutdown ou erro fatal de lifecycle.

### Concorrencia obrigatoria

O `WorkerManager` deve orquestrar goroutines diferentes no minimo para:
- loop/coordenador de cron jobs;
- loop/coordenador de consumers.

Podem existir goroutines adicionais por consumer ou worker concreto, desde que:
- sejam intencionais;
- sejam cancelaveis;
- nao vazem;
- participem do shutdown coordenado;
- tenham ownership claro no lifecycle.

### Shutdown obrigatorio

O `WorkerManager.Stop(ctx)` deve obrigatoriamente:

1. interromper a admissao de novo trabalho;
2. cancelar o contexto raiz do manager;
3. parar scheduler e loops de consumo de forma ordenada;
4. aguardar tarefas in-flight dentro de timeout configurado;
5. chamar stop/close dos consumers registrados;
6. consolidar e propagar erro explicitamente se algo falhar;
7. encerrar sem goroutine leak e sem abandono silencioso de trabalho em andamento.

Se houver conflito entre timeout e trabalho em andamento:
- a politica deve ser explicita;
- o erro deve ser retornado/logado;
- o shutdown nao pode fingir sucesso.

## Requisitos funcionais obrigatorios

1. Definir contrato compartilhado para lifecycle de workers/consumers.
2. Definir contrato compartilhado para jobs cron.
3. Definir `WorkerManager` como orquestrador central.
4. Definir `JobAdapter` como unico caminho valido para cron jobs.
5. Definir `ConsumerAdapter` como unico caminho valido para consumers.
6. Garantir que jobs e consumers rodem no mesmo binario worker.
7. Garantir startup e shutdown graciosos.
8. Garantir loops independentes para jobs e consumers.
9. Garantir que handlers de modulo continuem delegando para use cases.
10. Garantir que o transporte `memory` respeite a semantica persistida do outbox existente.

## Requisitos nao funcionais obrigatorios

1. Sem leak de goroutine.
2. Sem data race obvia.
3. Sem dependencia escondida.
4. Sem fallback silencioso que gere falso positivo operacional.
5. Logging estruturado com `log/slog`.
6. Pontos claros para metricas e tracing.
7. Cancelamento cooperativo com `context.Context`.
8. Timeout explicito de shutdown.
9. Politica explicita de overlap de cron job.
10. Clareza diagnostica de erro de startup, runtime e shutdown.

## Proibicoes explicitas

- Nao usar `uber/fx`, `fx.Lifecycle`, `dig` ou equivalente.
- Nao inicializar cron jobs fora do `WorkerManager`.
- Nao inicializar consumers fora do `WorkerManager`.
- Nao registrar cron job sem `JobAdapter`.
- Nao registrar consumer sem `ConsumerAdapter`.
- Nao colocar regra de negocio em `internal/platform`.
- Nao acessar repository direto a partir de job trigger ou consumer handler concreto.
- Nao tratar `memory` como fila volatil em RAM.
- Nao alterar internamente o outbox existente.
- Nao usar `_ = dependencia`, `_ = parametro` ou blank identifier para mascarar dependencia/import/parametro nao utilizado; o desenho deve ser corrigido na origem.
- Nao usar comentarios no codigo final.

## Estrutura final esperada do projeto

Use este desenho como referencia obrigatoria de destino. Adapte nomes finos ao repositorio apenas se houver forte justificativa, mas preserve a semantica.

```text
internal/
  platform/
    worker/
      manager.go
      config.go
      lifecycle.go
      errors.go
      types.go
      registry.go
      job/
        adapter.go
        types.go
        scheduler.go
      consumer/
        adapter.go
        registry.go
        types.go
        runner.go
        memory/
          adapter.go
        kafka/
          adapter.go
        rabbitmq/
          adapter.go
        sqs/
          adapter.go
        azureservicebus/
          adapter.go
```

Se o repositorio pedir uma organizacao mais enxuta, ainda assim o resultado final precisa deixar inequivoco:
- onde vive o `WorkerManager`;
- onde vivem os `JobAdapters`;
- onde vivem os `ConsumerAdapters`;
- onde fica o contrato compartilhado;
- onde ficam as implementacoes por tecnologia.

## Fluxo final esperado

```text
modulo concreto
  -> cria usecase
  -> cria executor de job ou handler de consumer
  -> passa executor para JobAdapter ou ConsumerAdapter
  -> bootstrap agrega adapters
  -> bootstrap instancia WorkerManager
  -> WorkerManager.Start(ctx)
  -> manager sobe goroutine de cron
  -> manager sobe goroutine de consumers
  -> signal/shutdown
  -> WorkerManager.Stop(ctx)
```

## Exemplo obrigatorio de contratos

```go
package worker

import "context"

type Managed interface {
    Name() string
    Start(context.Context) error
    Stop(context.Context) error
}

type Job interface {
    Name() string
    Schedule() string
    Run(context.Context) error
}

type Consumer interface {
    Name() string
    Technology() string
    Start(context.Context) error
    Stop(context.Context) error
}
```

## Exemplo obrigatorio de JobAdapter

```go
package job

import "context"

type Adapter struct {
    name     string
    schedule string
    run      func(context.Context) error
}

func NewAdapter(name string, schedule string, run func(context.Context) error) *Adapter {
    return &Adapter{
        name:     name,
        schedule: schedule,
        run:      run,
    }
}

func (a *Adapter) Name() string {
    return a.name
}

func (a *Adapter) Schedule() string {
    return a.schedule
}

func (a *Adapter) Run(ctx context.Context) error {
    return a.run(ctx)
}
```

## Exemplo obrigatorio de ConsumerAdapter

```go
package consumer

import "context"

type Handler interface {
    Handler(ctx context.Context, params map[string]string, body []byte) error
}

type Registry interface {
    Register(eventType string, handler Handler) error
}

type Adapter struct {
    name       string
    technology string
    runner     Runner
}

type Runner interface {
    Start(context.Context) error
    Stop(context.Context) error
}

func NewAdapter(name string, technology string, runner Runner) *Adapter {
    return &Adapter{
        name:       name,
        technology: technology,
        runner:     runner,
    }
}

func (a *Adapter) Name() string {
    return a.name
}

func (a *Adapter) Technology() string {
    return a.technology
}

func (a *Adapter) Start(ctx context.Context) error {
    return a.runner.Start(ctx)
}

func (a *Adapter) Stop(ctx context.Context) error {
    return a.runner.Stop(ctx)
}
```

## Exemplo obrigatorio de registro de handlers

```go
func registerBillingConsumers(registry consumer.Registry, handler consumer.Handler) error {
    if err := registry.Register("billing.invoice.generated", handler); err != nil {
        return err
    }

    return registry.Register("billing.invoice.failed", handler)
}
```

## Exemplo obrigatorio de bootstrap final

```go
jobs := []worker.Job{
    job.NewAdapter("billing-sync", cfg.CronBillingSync, billingSyncJob.Execute),
    job.NewAdapter("billing-reconciliation", cfg.CronBillingReconciliation, billingReconciliationJob.Execute),
}

consumers := []worker.Consumer{
    consumer.NewAdapter("billing-memory", "memory", memoryRunner),
    consumer.NewAdapter("billing-kafka", "kafka", kafkaRunner),
    consumer.NewAdapter("billing-rabbit", "rabbitmq", rabbitRunner),
}

manager := worker.NewManager(
    worker.Config{
        ShutdownTimeout: 30 * time.Second,
        Location:        mustLoadLocation("America/Sao_Paulo"),
    },
    jobs,
    consumers,
    logger,
)

if err := manager.Start(ctx); err != nil {
    return err
}
```

## Exemplo obrigatorio de leitura arquitetural do bootstrap

```text
1. modulo constroi use cases concretos;
2. modulo constroi executores e handlers concretos;
3. cron jobs entram exclusivamente via JobAdapter;
4. consumers entram exclusivamente via ConsumerAdapter;
5. bootstrap agrega tudo;
6. WorkerManager recebe tudo;
7. WorkerManager sobe cron e consumers no startup;
8. WorkerManager centraliza start, stop e graceful shutdown.
```

## Criterios de aceitacao inegociaveis

1. Existe um unico `WorkerManager` centralizando lifecycle.
2. Todo cron job obrigatoriamente respeita `JobAdapter`.
3. Todo consumer obrigatoriamente respeita `ConsumerAdapter`.
4. Jobs e consumers sobem no startup pelo `WorkerManager`.
5. Jobs e consumers rodam em goroutines distintas.
6. `WorkerManager` possui `Start` e `Stop`.
7. Ha cancelamento cooperativo e timeout de shutdown.
8. Nao ha bypass arquitetural dos adapters.
9. `internal/platform` continua sem regra de negocio.
10. `memory` continua significando transporte local persistido por outbox/banco.
11. O bootstrap final deixa inequivoco o papel central do manager.
12. A resposta final deve explicar onde cada responsabilidade ficou, como o lifecycle funciona e como o desenho evita falso positivo operacional.

## Resultado esperado da resposta

1. Explicar brevemente o desenho escolhido antes de codar.
2. Implementar a estrutura completa.
3. Mostrar como o bootstrap final registra jobs e consumers.
4. Demonstrar como start e stop funcionam.
5. Garantir aderencia a arquitetura do repositorio.
6. Entregar resumo final objetivo em PT-BR.
```
