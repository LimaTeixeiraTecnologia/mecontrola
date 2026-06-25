# Tarefa 6.0: Remover AgentModuleOption com dependências explícitas

<critical>Ler o plano-fonte `docs/plans/2026_06_24_arquitetura_agente_mastra_workflows_bounded_contexts.md` (Item 5) e a seção "Padrão Obrigatório de Módulo" em `AGENTS.md` antes de iniciar.</critical>

## Visão Geral

`NewAgentModule` recebe `opts ...AgentModuleOption` (`WithSessionStore`, `WithOutboxPublisher`, `WithOnboardingLLM`), padrão que `internal/identity`/`internal/billing` proíbem. Converter para DI explícita por campos nomeados (struct `AgentModuleDeps`), preservando o caráter genuinamente opcional do onboarding-LLM (que só `cmd/server` injeta; `cmd/worker` não).

<requirements>
- `AgentModuleOption` removido; dependências viram explícitas (preferir struct `AgentModuleDeps` com campos nomeados, não lista posicional gigante).
- `WithSessionStore`/`WithOutboxPublisher` (passadas por ambos os callers) → campos obrigatórios; `prepareSessionStore` tolerância a nil reavaliada.
- `WithOnboardingLLM` → campo explicitamente opcional/nullable (`*OnboardingLLMUseCases` ou struct com presença detectável), nil = modo determinístico. Nunca obrigatório (worker/e2e passariam zero-value mascarado).
- Comportamento observável preservado: server em modo LLM; worker em modo determinístico; error-paths do e2e idênticos.
- Esta é melhoria de consistência (escopo literal da regra hard é identity/billing) — registrar isso, não afirmar violação inexistente.
- Zero comentários em Go de produção (R-ADAPTER-001.1).
</requirements>

## Subtarefas

- [ ] 6.1 Definir `AgentModuleDeps` (campos nomeados) e reescrever `NewAgentModule` sem variadic.
- [ ] 6.2 Atualizar `cmd/server/server.go` (3 deps) e `cmd/worker/worker.go` (2 deps, onboarding-LLM nil) e `internal/agent/e2e/module_test.go`.
- [ ] 6.3 Preservar a degradação graciosa de `onboardingLLMUnavailable` quando o campo opcional for nil.
- [ ] 6.4 Remover `agentModuleBuilder`/options órfãos.

## Detalhes de Implementação

Ver plano-fonte (Item 5). Âncoras: `internal/agent/module.go` (`NewAgentModule` :149, options :80-113, `attachOnboardingLLM`/`onboardingLLMUnavailable` ~:658, `prepareSessionStore`), `cmd/server/server.go:226`, `cmd/worker/worker.go:196`, `internal/agent/e2e/module_test.go`. Pré-condição: Task 1.0 (também toca `module.go` — serializar).

## Critérios de Sucesso

- `NewAgentModule` sem variadic; deps explícitas e nomeadas.
- Onboarding-LLM continua opcional (worker/e2e sem zero-value mascarado).
- Os três call-sites compilam; error-paths do e2e preservados.

## Definition of Done (DoD)

1. `AgentModuleOption` e `With*` removidos.
2. `WithSessionStore`/`WithOutboxPublisher` viram obrigatórios; onboarding-LLM nullable explícito.
3. server/worker/e2e atualizados e compilando; comportamento preservado.
4. Build dos dois entrypoints + suites verdes.

## Critérios de Aceite (gates executáveis)

```bash
cd /Users/jailtonjunior/Git/mecontrola

grep -rn "AgentModuleOption\|WithSessionStore\|WithOutboxPublisher\|WithOnboardingLLM\|agentModuleBuilder" \
  internal/agent/ cmd/ --include="*.go" \
  && echo "REVISAR: forma de options remanescente" || echo "OK"

go build ./... && go build ./cmd/server/... ./cmd/worker/...

go test ./internal/agent/e2e/... -run TestNewAgentModule

grep -n "^[[:space:]]*//" internal/agent/module.go | grep -Ev "(//go:|//nolint:|// Code generated)" \
  && echo "FAIL: comentários proibidos" || echo "OK"
```

## Skills Necessárias

- `go-implementation` — mudança de assinatura de construtor e wiring em Go (Etapas 1–5 + checklist R0–R7).
- `mastra` — módulo do `internal/agent` (composição Thread→Run, kernel, onboarding).

## Testes da Tarefa

- [ ] Testes unitários: construção com onboarding-LLM nil → modo determinístico; com presente → modo LLM.
- [ ] Testes de integração: error-paths do e2e (`ErrAPIKeyRequired`, gateway nil, transactions disabled) inalterados.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`.</critical>

## Arquivos Relevantes
- `internal/agent/module.go`
- `cmd/server/server.go`, `cmd/worker/worker.go`
- `internal/agent/e2e/module_test.go`
