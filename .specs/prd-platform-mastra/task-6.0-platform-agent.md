# Tarefa 6.0: Primitivo Agent em `internal/platform/agent`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar `internal/platform/agent`: o primitivo genérico de agent equivalente a `@mastra/core/agent`. Inclui `AgentRegistry`/`WorkflowRegistry`, `AgentRuntime` (resolve Thread→abre Run auditável), `Agent.Execute` (síncrono) e `Agent.Stream` (streaming com validação fim-de-stream), hooks de ciclo de vida, structured output, `RuntimeContext` e persistência de `platform_runs`. Consome llm (1.0), tool (3.0), kernel (4.0) e memory (5.0).

<requirements>
- RF-01: registrar/resolver agentes por id; ciclo de vida explícito.
- RF-02: hooks de ciclo de vida (pré/pós execução, em torno de tool).
- RF-03: execução síncrona.
- RF-07: validação de structured output na conclusão do stream.
- RF-08: falha explícita e auditável quando o contrato não conforma.
- RF-26/RF-27: runtime context tipado acessível, não persistido.
- RF-28: Run auditável (correlação, status fechado, duração, erro).
- RF-29: tracing correlacionável reutilizando a stack O11Y existente.
- RF-30: métricas com cardinalidade controlada (enums fechados; sem `resource_id`/`thread_id`).
- RF-31/RF-32: sem regra de domínio na plataforma; consumível por múltiplos módulos.
- RF-33: `RunStatus`/`ToolOutcome`/`AwaitingKind`/`ExecutionMode` como tipos fechados.
- RF-34: LLM só no step de parse; kernel sem LLM.
- Thread-first (`resourceId`/`threadId`); roteamento `Workflow→Tool→binding→usecase` (R-AGENT-WF-001).
</requirements>

## Subtarefas

- [ ] 6.1 Tipos fechados `agent.RunStatus`, `ToolOutcome`, `AwaitingKind`, `ExecutionMode` (com `String()`/`IsValid()`/`Parse*`).
- [ ] 6.2 `AgentRegistry`/`WorkflowRegistry` (registro/resolução por id; sem switch de domínio).
- [ ] 6.3 `AgentRuntime.Execute`: `ThreadGateway.GetOrCreate` → abre Run (`platform_runs`) → injeta RuntimeContext → resolve `Definition[S]` → `Engine.Start/Resume` → fecha Run auditável.
- [ ] 6.4 `Agent.Execute` (síncrono) com structured output validado na fronteira; binding de tools e de memory (working memory no system prompt).
- [ ] 6.5 `Agent.Stream` (deltas via `llm.Stream`; `Result(ctx)` valida o contrato na conclusão do stream — ADR-003).
- [ ] 6.6 Hooks de ciclo de vida; persistência/auditoria de Run; métricas (`agent_runs_total`, `agent_stream_total`, `agent_tool_invocations_total`) com labels fechados.

## Detalhes de Implementação

Ver techspec.md "Interfaces Chave > Agent/AgentRuntime", "Fluxo de dados", ADR-002/003/007, e `.claude/rules/agent-workflows-tools.md` (re-escopado). Tool/Workflow finos: sem SQL/regra/branching de domínio. DTOs de input com `Validate()` (R-DTO-VALIDATE-001).

## Critérios de Sucesso

- Execução síncrona e streaming funcionam sob o mesmo contrato lógico; structured output validado (sync e fim-de-stream) com falha explícita testada.
- Todo `Execute` resolve Thread e produz Run auditável; métricas sem labels de alta cardinalidade.
- Tipos de estado são fechados; nenhuma `string` solta na fronteira.
- Gates: `grep -rn "^[[:space:]]*//" internal/platform/agent/ --include="*.go" --exclude-dir=mocks --exclude="*_test.go" | grep -Ev "(//go:|//nolint:|// Code generated)"` vazio; `grep -rn '"resource_id"\|"thread_id"\|"correlation_key"' internal/platform/agent/ --include="*.go" --exclude="*_test.go"` vazio.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `go-implementation` — implementação Go obrigatória e inegociável (CLAUDE.md) do primitivo agent e testes.
- `mastra` — Agent/Thread→Run/Tool/Workflow são o núcleo do padrão Mastra portado (skill canônica).

## Testes da Tarefa

- [ ] Testes unitários (testify/suite whitebox): registry, AgentRuntime Thread→Run, Execute síncrono, Stream fim-de-stream conforme/não-conforme, hooks, tipos fechados (`IsValid`/`Parse`).
- [ ] Testes de integração (`//go:build integration`, testcontainers Postgres+pgvector): Run auditável persistido em `platform_runs`; suspend/resume via kernel.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/platform/agent/` (novo) — domain/application/infrastructure.
- `internal/platform/llm/`, `internal/platform/tool/`, `internal/platform/memory/`, `internal/platform/workflow/` — consumidos.
- `.claude/rules/agent-workflows-tools.md` — contrato re-escopado.
