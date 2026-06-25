# Tarefa 1.0: Estabelecer kernel como caminho padrão e obrigatório

<critical>Ler o plano-fonte `docs/plans/2026_06_24_arquitetura_agente_mastra_workflows_bounded_contexts.md` (seção "Refatorações e Remoções Necessárias") antes de iniciar — sua tarefa será invalidada se você pular.</critical>

## Visão Geral

Tornar o kernel genérico (`internal/platform/workflow`) o caminho **padrão e obrigatório** para writes de transação e para o gate HITL, eliminando a condição em que o caminho legacy permanece como fallback vivo. Esta é a pré-condição que destrava as Tasks 2.0/3.0/4.0: enquanto `WorkflowKernelConfig.TransactionsWriteEnabled` puder ser `false` ou `confirmEngine` puder ser `nil`, remover legacy regride o comportamento.

<requirements>
- `TransactionsWriteEnabled` passa a ser `true` por padrão em produção (server e worker), ou a guarda `if ...Engine != nil` deixa de ter ramo legacy alcançável em produção.
- `confirmEngine` (kernel `destructive_confirm`) torna-se dependência obrigatória do agent: ausência é erro de wiring na construção, não degradação silenciosa.
- Nenhuma regra de domínio, SQL ou branching de domínio introduzido no kernel (R-WF-KERNEL-001.1/.2).
- Zero comentários em Go de produção (R-ADAPTER-001.1).
</requirements>

## Subtarefas

- [ ] 1.1 Definir `TransactionsWriteEnabled` default `true` no carregamento de config (`configs/config.go`) e confirmar propagação em `cmd/server` e `cmd/worker`.
- [ ] 1.2 Tornar `confirmEngine` obrigatório em `internal/agent/module.go` (`module.go:511`): falhar a construção do módulo se store/factory ausentes, em vez de seguir com `confirmEngine == nil`.
- [ ] 1.3 Ajustar `dispatchWrite` (`daily_ledger_agent.go`) para que kinds de escrita e destrutivos nunca dependam de um ramo `Engine == nil`/`confirmEngine == nil` em produção.
- [ ] 1.4 Atualizar testes que constroem o módulo sem kernel/confirm para o novo contrato (erro explícito esperado).

## Detalhes de Implementação

Ver plano-fonte, itens 2/3/4 da seção "Refatorações". Âncoras: `internal/agent/module.go:511` (wiring de `confirmEngine`), `:522` (`TransactionsWriteEnabled`), `internal/agent/application/services/daily_ledger_agent.go` (`dispatchWrite` ~:263). Esta task **não** remove código legacy — apenas garante que o kernel é o caminho ativo; a remoção ocorre em 2.0/3.0/4.0.

## Critérios de Sucesso

- Em produção, todo write de transação roteia pelo kernel (`Engine[ExpenseState]`) e todo destrutivo/sensível pelo `confirmEngine`.
- Construir o agent sem store/factory de kernel resulta em erro de wiring determinístico.
- `go build ./...` e `go test ./internal/agent/... ./internal/platform/workflow/...` verdes.

## Definition of Done (DoD)

1. `TransactionsWriteEnabled` é `true` por padrão (ou a flag é removida e o kernel é incondicional).
2. `confirmEngine` é não-opcional: não existe caminho de produção em que ele seja `nil` e um destrutivo execute mesmo assim.
3. Nenhum teste de produção depende de `TransactionsWriteEnabled=false` para passar (cenários legacy isolados em testes explicitamente marcados ou removidos).
4. Build, vet e suites do agent/kernel verdes.

## Critérios de Aceite (gates executáveis)

```bash
cd /Users/jailtonjunior/Git/mecontrola

# A flag não força mais o ramo legacy: default true (ajustar grep ao nome real do campo)
grep -rn "TransactionsWriteEnabled" configs/ cmd/ internal/agent/module.go --include="*.go"

# confirmEngine não tem ramo silencioso de bypass em produção
grep -n "confirmEngine == nil\|confirmEngine != nil" internal/agent/application/services/daily_ledger_agent.go

# Kernel permanece sem import de domínio (R-WF-KERNEL-001.1)
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "internal/agent\|internal/transactions\|internal/billing\|internal/identity" \
  internal/platform/workflow/ && echo "FAIL: import de domínio no kernel" || echo "OK"

# Zero comentários em Go de produção do agent
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" "^[[:space:]]*//" \
  internal/agent/ | grep -Ev "(//go:|//nolint:|// Code generated)" \
  && echo "FAIL: comentários proibidos" || echo "OK"

# Build + suites
go build ./... && go test ./internal/agent/... ./internal/platform/workflow/...
```

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por create-tasks Etapa 4.1. -->

- `go-implementation` — alteração de wiring/config em Go de produção exige as Etapas 1–5 e o checklist R0–R7.
- `mastra` — toca o ciclo Thread→Run e o consumo do kernel pelo `internal/agent` (regra R-AGENT-WF-001 / R-WF-KERNEL-001).

## Testes da Tarefa

- [ ] Testes unitários: construção do módulo sem store/factory → erro de wiring.
- [ ] Testes de integração: write de transação e destrutivo roteiam pelo kernel com a config default.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`.</critical>

## Arquivos Relevantes
- `configs/config.go` — default de `TransactionsWriteEnabled`.
- `internal/agent/module.go` — wiring de `confirmEngine` e kernel (`:511`, `:522`).
- `internal/agent/application/services/daily_ledger_agent.go` — `dispatchWrite`.
- `cmd/server/server.go`, `cmd/worker/worker.go` — propagação de config.
