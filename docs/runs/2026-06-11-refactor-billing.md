# Refactor `internal/billing` — Plano

## Context

`internal/billing` concentra hoje 4 hotspots com complexidade acidental alta:

- `process_kiwify_webhook.go` (360 LOC): parser de payload Kiwify, parser custom de timestamp, extração de funnel token/carrier, builders de DTO inline e switch de dispatch de 6 casos, tudo em um único arquivo.
- `module.go` (219 LOC): bootstrap correto, mas carrega validação de planos (42 LOC), mapeamento de config do client Kiwify (18 LOC) e registro implícito de event handlers misturado ao wiring.
- `kiwify_webhook_handler.go` (87 LOC): handler tecnicamente fino (sem regra de domínio), mas com 21 LOC de error-switch repetitivo que pode virar tabela.
- `reconcile_subscriptions.go` (131 LOC): loop de paginação acumula roteamento por status (15 LOC) e resolução status→trigger (28 LOC) que são funções puras passíveis de extração.

Objetivo: reduzir complexidade acidental usando **factories concretas, value objects e funções puras locais** — sem criar interfaces novas, sem abstrair webhook genericamente, sem mover regra para handler/job/producer. Preservar idempotência, observabilidade, semântica pública e thinness dos adapters (R-ADAPTER-001).

Skills a carregar antes de editar: `AGENTS.md`, `.claude/skills/go-implementation/SKILL.md`, `architecture.md`, `interfaces.md`, `examples-domain-flow.md`. `testing.md` apenas se houver reescrita estrutural das suites — a intenção é preservar as suites existentes ajustando apenas chamadas dos construtores. Limite de 4 referências simultâneas respeitado.

## Recomendação

### 1. Extrair payload Kiwify para subpacote dedicado

Criar `internal/billing/application/usecases/kiwifypayload/` (subpacote do mesmo agregado de aplicação — permanece na fronteira application, não vaza para domain).

Arquivos novos:

- `kiwifypayload/payload.go` — `Payload` struct (ex-`kiwifyWebhookPayload`) e `Decode([]byte) (Payload, error)`.
- `kiwifypayload/time.go` — `kiwifyTime` + `UnmarshalJSON` com cadeia RFC3339Nano → RFC3339 → BRT (puro, testável isoladamente).
- `kiwifypayload/tracking.go` — `func ExtractFunnel(p Payload) (token string, carrier string, present bool)` consolidando a lógica SCK/S1/Src hoje em `process_kiwify_webhook.go:119-144` + `funnel_token.go:extractFunnelToken`.
- `kiwifypayload/commands.go` — uma factory pura por evento: `ToSaleApprovedInput`, `ToSubscriptionRenewedInput`, `ToSubscriptionLateInput`, `ToSubscriptionCanceledInput`, `ToRefundOrChargebackInput`. Cada uma recebe `Payload` e retorna o DTO de input concreto da camada `application/dtos/input`.
- `kiwifypayload/event.go` — `Trigger` (string newtype) + `func ClassifyTrigger(p Payload) Trigger` com fallback de abandoned cart hoje em `eventType()`.

Resultado: `process_kiwify_webhook.go` deixa de hospedar parsing e fica focado em orquestração (auditoria, telemetria, dispatch).

### 2. Strategy map explícito de dispatch (apenas se reduzir branching real)

Em `process_kiwify_webhook.go`, substituir o switch de 6 casos por uma tabela registrada no construtor:

```go
type dispatchEntry struct {
    name    string
    execute func(ctx context.Context, p kiwifypayload.Payload) error
}
```

`handlers := map[kiwifypayload.Trigger]dispatchEntry{ ... }` populado uma vez em `NewProcessKiwifyWebhook`. Dispatch vira lookup + chamada. Eventos noop (`billet_created`, `pix_created`) entram como entradas explícitas com `execute = nil` — telemetria continua emitindo `received` mas pula execução.

Justificativa para introduzir: reduz 53 LOC de switch + DTO inline para ~6 entradas declarativas; o branching real é igual ao número de eventos suportados (não há generalização especulativa).

### 3. Catálogo de planos como componente concreto

Extrair `ensurePlansConfigured` + `planMissingEnvVars` de `module.go` (linhas 166-207) para `internal/billing/infrastructure/config/billing/plan_catalog.go`:

- `type PlanCatalog struct { plans map[valueobjects.PlanCode]PlanBinding }`
- `func NewPlanCatalog(cfg *configs.Configuration) (*PlanCatalog, error)` — concentra validação de env vars + criação dos `valueobjects.Plan`.
- Métodos: `LookupByProductID(productID string) (entities.Plan, bool)`, `All() []entities.Plan`.

`module.go` reduz a `catalog, err := billingconfig.NewPlanCatalog(cfg); planLookup := catalog.LookupByProductID`. Bootstrap perde 42 LOC e a regra incidental.

### 4. Compactar `status.go` com tabela única

Substituir os três switches paralelos por uma tabela única indexada por `Status`:

```go
var statusTable = map[Status]struct {
    wire             string
    activeForBilling bool
    terminal         bool
}{ ... }
```

`String()`, `IsActiveForBilling()`, `IsTerminal()`, `ParseStatus()` viram lookups O(1). Reduz 75 → ~45 LOC, elimina divergência entre listas de cases. Mantém superfície pública idêntica.

### 5. Compactar error mapper do handler

Em `kiwify_webhook_handler.go`, substituir o switch de 6 erros por tabela:

```go
type errorResponse struct{ status int; code string; detail string }
var errorTable = []struct{ target error; resp errorResponse }{
    {usecases.ErrInvalidPayload, ...}, ...
}
```

Handler permanece thin: extrai contexto → chama usecase → mapeia erro via tabela usando `errors.Is`. Sem regra de domínio nova.

### 6. Extrair resolução de ação de reconciliação

Em `reconcile_subscriptions.go`, mover o switch status→trigger (linhas 104-131) para uma função pura `resolveReconcileAction(sale kiwify.Sale) (action reconcileAction, ok bool)` no mesmo arquivo, onde `reconcileAction` é um pequeno enum interno (saleApproved | refundOrChargeback). Loop principal vira `action, ok := resolveReconcileAction(sale); if !ok { continue }; action.execute(ctx, ...)`.

### 7. NÃO refatorar agora

- `subscription_event_publisher.go` — já é puro serialize+publish, sem decisão. Tocar só agregaria churn.
- `plan.go` — 60 LOC, constructor com validação clara. Manter; `PlanCatalog` cobre a regra incidental que vazava para `module.go`.

### Programação funcional aplicada

Apenas como apoio local:

- `kiwifypayload` é um pipeline puro `[]byte → Payload → Trigger → DTO`. Cada estágio é função sem efeito colateral.
- Telemetria, auditoria e dispatch permanecem como efeitos explícitos no usecase — funcional não esconde efeitos.

## Arquivos críticos

Modificar:

- `internal/billing/application/usecases/process_kiwify_webhook.go` — reduzir a orquestração; remover parser/builders inline.
- `internal/billing/application/usecases/process_kiwify_webhook_test.go` — ajustar construtores; manter cenários.
- `internal/billing/application/usecases/reconcile_subscriptions.go` — extrair `resolveReconcileAction`.
- `internal/billing/module.go` — substituir bloco de planos por `PlanCatalog`; manter wiring restante.
- `internal/billing/domain/valueobjects/status.go` — tabela única.
- `internal/billing/domain/valueobjects/funnel_token.go` — consolidar lógica de extração (ou mover para `kiwifypayload/tracking.go` e deletar duplicação).
- `internal/billing/infrastructure/http/server/handlers/kiwify_webhook_handler.go` — error table.

Criar:

- `internal/billing/application/usecases/kiwifypayload/{payload,time,tracking,commands,event}.go`
- `internal/billing/infrastructure/config/billing/plan_catalog.go`

Padrão repetido (5 arquivos novos do `kiwifypayload`): funções puras sem dependência de framework, sem comentários (R-ADAPTER-001.1), apenas erros com `fmt.Errorf("ctx: %w", err)` e `errors.Join` quando agregar.

## Reutilização

- `valueobjects.FunnelToken` (`funnel_token.go`) — fonte canônica para o token; `kiwifypayload.ExtractFunnel` retorna o valor bruto e o usecase chama `valueobjects.NewFunnelToken` como hoje.
- `valueobjects.Status`, `valueobjects.Plan`, `valueobjects.PlanCode` — sem alteração de superfície pública.
- `outbox.Publisher` em `subscription_event_publisher.go` — sem alteração.
- DTOs em `application/dtos/input/` — sem alteração; `kiwifypayload/commands.go` retorna exatamente esses tipos.
- `interfaces.RepositoryFactory`, `observability.Observability` — sem alteração.

## Verificação

Antes de declarar pronto, executar Checklist R0–R7 de `references/build.md` e:

1. Build: `go build ./internal/billing/...`
2. Vet: `go vet ./internal/billing/...`
3. Testes unit existentes: `go test ./internal/billing/...` (todas as suites atuais devem passar sem alteração de cenário)
4. Gate R-ADAPTER-001.1 (zero comentários):
   ```bash
   grep -rn --include="*.go" --exclude-dir=mocks --exclude="*.pb.go" --exclude="*_test.go" \
     "^[[:space:]]*//" internal/billing/ \
     | grep -Ev "(//go:|//nolint:|// Code generated)"
   ```
   Deve retornar vazio.
5. Gate R-ADAPTER-001.2 (sem SQL direto em adapter):
   ```bash
   grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
     "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|db\.Exec" \
     internal/billing/infrastructure/http/server/handlers/ \
     internal/billing/infrastructure/messaging/database/consumers/ \
     internal/billing/infrastructure/messaging/database/producers/ \
     internal/billing/infrastructure/jobs/handlers/
   ```
   Deve retornar vazio.
6. Race: `go test -race ./internal/billing/application/usecases/...`
7. Suite de webhook end-to-end: validar manualmente um payload Kiwify real (sandbox) — confirmar ordem de telemetria (received → tracking carrier → signature) e que envelope ID + funnel token são extraídos idênticos ao baseline.

Critérios de aceitação (replicados do prompt original):

- `process_kiwify_webhook.go` < ~180 LOC, apenas orquestração.
- `module.go` < ~160 LOC, sem validação de planos inline.
- Suites existentes passam sem alteração de cenário (ajustes apenas em setup).
- Nenhuma interface nova introduzida sem justificativa concreta.

## Nota sobre memória de plano

Memória `feedback_save_plans_to_docs_runs` instrui replicar este plano para `docs/runs/2026-06-11-refactor-billing.md`. Plan mode bloqueia escrita fora do arquivo de plano designado — após aprovação via ExitPlanMode, replicarei como primeira ação.
