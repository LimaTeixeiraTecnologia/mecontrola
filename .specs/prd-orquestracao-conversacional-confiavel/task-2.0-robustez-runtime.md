# Tarefa 2.0: Robustez do runtime — truncamento, falha-segura e observabilidade

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Endurecer `internal/platform/agent` por extensão aditiva (sem reescrever o substrato; kernel
`internal/platform/workflow` intocado): truncamento por length vira estado fechado com falha-segura, o
teto de tokens fica configurável, e os gaps de persistência hoje silenciosos passam a ser observáveis.

<requirements>
- RF-23/RF-47: adicionar `ToolOutcomeTruncated` ao enum fechado `ToolOutcome` (`types.go`,
  String/IsValid/ParseToolOutcome); o runtime consulta `result.TruncatedByLength` e, quando `true`,
  marca `RunStatus=Failed`, `ToolOutcome=ToolOutcomeTruncated` e fallback seguro.
- RF-22: erro/vazio/truncamento de tool → run `failed` + fallback curto sem detalhe técnico, sem
  confirmação de sucesso/valor/categoria inventado.
- RF-24: teto de tokens elevado e configurável via `AGENT_MECONTROLA_MAX_TOKENS` (default 3072) passado a
  `WithDefaultMaxTokens`.
- RF-25: erro de `MessageStore.Append` → métrica `agent_message_append_errors_total{agent_id, role}` +
  log.
- RF-26: erro de `RunStore.Update` → log Error + `agent_run_update_errors_total{agent_id}`; não reportar
  sucesso de run cujo estado não persistiu (não incrementar `agent_runs_total` nesse caso).
- RF-27: agregar erros de múltiplas tools em `errStr` (bounded, sanitizado), não só o primeiro.
- RF-28: run sem scorer / sem mensagem persistida observáveis (métrica + log).
- RF-33: métricas com cardinalidade fechada (sem `user_id`/`thread_id`/`resource_id`).
</requirements>

## Subtarefas

- [ ] 2.1 `types.go`: adicionar `ToolOutcomeTruncated` (+ `String()="truncated"`, `IsValid`, caso em
  `ParseToolOutcome`).
- [ ] 2.2 `runtime.go`: consultar `result.TruncatedByLength` no bloco de decisão de status; marcar
  Failed + `ToolOutcomeTruncated` + `errStr`; emitir `agent_run_truncated_total{agent_id}`.
- [ ] 2.3 `runtime.go`: observar `MessageStore.Append` (métrica por `role`) e `RunStore.Update` (erro +
  métrica; não emitir `agent_runs_total` de sucesso quando o Update falhar).
- [ ] 2.4 `runtime.go`: agregar múltiplos erros de tool em `errStr` de forma bounded e sanitizada.
- [ ] 2.5 Wiring/`mecontrola_agent.go`: resolver `AGENT_MECONTROLA_MAX_TOKENS` (default 3072) →
  `WithDefaultMaxTokens`.
- [ ] 2.6 Fallback seguro: texto curto determinístico em PT-BR para falha/truncamento/vazio.

## Detalhes de Implementação

Ver `adr-002-runtime-robustez-truncamento.md` e `techspec.md` → "Monitoramento e Observabilidade".
Interfaces reais: `Result{Content, ToolOutcome, ToolCalls, TruncatedByLength}` (`agent/ports.go:56`);
`closeRun(ctx, run, status, outcome, errStr, start)` (`runtime.go:301`); `_ = r.runs.Update` em
`runtime.go:308`; enum `ToolOutcome` iota+1 em `types.go:48`. Mudanças aditivas — não alterar contrato
público nem o kernel de workflow.

## Critérios de Sucesso

- Truncamento por length → run `failed` + `ToolOutcomeTruncated` + fallback seguro; nunca sucesso
  silencioso.
- Falha de `Update`/`Append` emite métrica e log; `agent_runs_total` não reporta sucesso não persistido.
- Teto de tokens configurável por env; default 3072.
- `go build/vet/test -race` verdes; kernel `internal/platform/workflow` inalterado.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — altera o runtime da plataforma agente (Thread→Run, outcome, observabilidade) do stack mecontrola.
- `domain-modeling-production` — modela `ToolOutcomeTruncated` como estado fechado (state-as-type), estados ilegais irrepresentáveis.

## Testes da Tarefa

- [ ] Testes unitários: truncamento → Failed + `ToolOutcomeTruncated` + fallback; `Update` falho não
  reporta sucesso; `Append` falho emite métrica; agregação de múltiplos erros de tool; parse do enum.
- [ ] Testes de integração: não aplicável nesta tarefa (coberto em 7.0/8.0).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/platform/agent/types.go`
- `internal/platform/agent/runtime.go`
- `internal/agents/application/agents/mecontrola_agent.go`
- `internal/agents/module.go` (wiring do env)
