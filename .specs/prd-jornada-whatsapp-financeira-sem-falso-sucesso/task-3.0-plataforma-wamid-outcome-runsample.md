# Tarefa 3.0: Plataforma — WAMID na fronteira e outcome no RunSample

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Esta é a FUNDAÇÃO de plataforma consumida pela tarefa 4.0 (scorer de persistência). Ela fecha duas superfícies de falso sucesso na fronteira do substrato Mastra, sem introduzir GoF pattern: (1) tornar o run sem WAMID irrepresentável — `InboundRequest.Validate()` passa a rejeitar `MessageID == ""` com sentinela tipada, e o span `agent.runtime.execute` passa a carregar `wamid`/`workflow` padronizados; (2) propagar o efeito da tool como DADO até o `RunSample` — `scorer.ToolCallRecord` ganha campo `Outcome string` (STRING para não importar `internal/platform/agent` no kernel scorer, preservando o layering) e `ScoringHooks.AfterTool` deixa de descartar tanto `resultBytes` quanto tools com erro Go, para que o scorer da 4.0 enxergue escrita sem efeito. Cobre RF-13, RF-17 (parcial — apenas a propagação; a política de score é da 4.0) e RF-22 (parcial — apenas span). Detalhes em `techspec.md`, `adr-005-correlacao-wamid-e-run-update-observavel.md` (parte fronteira) e `adr-004-scorer-persistencia-per-run.md` (parte propagação) — não duplicar aqui.

<requirements>
- RF-13, RF-17 (parcial — propagação), RF-22 (parcial — span) conforme `prd.md`.
- Decisões firmes do ADR-005 (item 1: fronteira única + sentinela; item 5: padronização de span) e ADR-004 (itens 1 e 2: campo aditivo `Outcome string` + `AfterTool` para de descartar `resultBytes` e `err`).
- Sem novo design pattern GoF — validação de fronteira + campo aditivo + desserialização genérica.
- DMMF: estado ilegal irrepresentável (run sem WAMID rejeitado antes de `runs.Insert`); `errors.Join` preservado em `Validate()`.
- Layering: `internal/platform/scorer` NÃO importa `internal/platform/agent`; `Outcome` é `string` no kernel scorer, semântica fechada mora em `internal/agents/application/scorers` (tarefa 4.0).
- Zero comentários em Go de produção (R-ADAPTER-001.1); manter comentários HTML guard-rail deste arquivo.
- Cardinalidade de métrica controlada (R-TXN-004 / R-AGENT-WF-001.5): span carrega `wamid`, mas nenhum label de métrica pode usar WAMID.
</requirements>

## Subtarefas

- [ ] 3.1 (ADR-005 — fronteira) Adicionar sentinela `ErrEmptyMessageID` em `internal/platform/agent/errors.go` (junto de `ErrEmptyAgentID`/`ErrEmptyMessage`) e, em `InboundRequest.Validate()` (`internal/platform/agent/ports.go` ~L73-88), a branch `if i.MessageID == "" { errs = append(errs, fmt.Errorf("message_id: %w", ErrEmptyMessageID)) }`, preservando `errors.Join`. Torna o run sem WAMID irrepresentável (falha antes de `runs.Insert`, cobrindo todos os callers presentes e futuros).
- [ ] 3.2 (ADR-005 — span) Padronizar o span `agent.runtime.execute` (`internal/platform/agent/runtime.go` ~L88) adicionando `observability.String("wamid", in.MessageID)` e `observability.String("workflow", ...)` às atributos, alinhado aos campos padronizados `run_id`/`wamid`/`workflow`/`stage`/`status` (RF-22).
- [ ] 3.3 (ADR-004 — campo aditivo) Adicionar campo `Outcome string` a `scorer.ToolCallRecord` (`internal/platform/scorer/scorer.go` ~L10-14). STRING deliberado: NÃO importar `internal/platform/agent` no kernel scorer (preserva layering). Zero-value `""` não quebra scorers existentes.
- [ ] 3.4 (ADR-004 — AfterTool para de descartar `resultBytes`) Em `ScoringHooks.AfterTool` (`internal/agents/application/agents/scoring_hooks.go` ~L83), deixar de descartar o 4º param `resultBytes`: implementar função `extractOutcome(resultBytes)` que desserializa `{"outcome":string}` de forma genérica (`""` se ausente, ex. read-tools) e gravar o valor em `ToolCallRecord.Outcome`.
- [ ] 3.5 (ADR-004 — AfterTool para de descartar `err`) Remover o `if err != nil { return }` incondicional (~L84): quando o `exec` retornar erro Go, registrar o `ToolCallRecord` com marcador de outcome de erro (ex. `"usecaseError"`) para o scorer enxergar a falha, sem sumir do sample.
- [ ] 3.6 (Testes) Ver seção "Testes da Tarefa".

## Detalhes de Implementação

Ver `techspec.md`, `adr-005-correlacao-wamid-e-run-update-observavel.md` (Decisão itens 1 e 5) e `adr-004-scorer-persistencia-per-run.md` (Decisão itens 1 e 2; Riscos R1/R2/R3) desta pasta — **referenciar em vez de duplicar**. ADR-005 fixa a defesa na fronteira única (DMMF estado ilegal irrepresentável) e a padronização de campos de span. ADR-004 fixa o campo aditivo `Outcome string` (STRING por layering — R3/R(d)) e a obrigatoriedade de `AfterTool` parar de descartar TANTO `resultBytes` QUANTO `err` (R2: se apenas um for tratado, o scorer permanece cego a parte das falhas). RISCO (ADR-005 R2): varrer `*_test.go` do runtime/consumers e preencher `MessageID` nos fixtures de `InboundRequest`, senão quebram ao ligar a validação. RISCO (ADR-004 R2 / tasks.md): `AfterTool` precisa parar de descartar `resultBytes` E `err` — senão o scorer da 4.0 fica cego ao pior caso (escrita aceita sem efeito). Esta tarefa NÃO cria o scorer `write_persistence_accuracy` nem endurece `no_hallucination` (isso é da 4.0); entrega apenas a propagação e a fronteira.

## Critérios de Sucesso

- `InboundRequest{MessageID:""}.Validate()` ⇒ erro nomeando `message_id` (via sentinela `ErrEmptyMessageID`), agregado por `errors.Join`.
- Todo run financeiro `routed` nasce com `correlation_key == WAMID` (a validação barra o vazio antes de `runs.Insert`); span `agent.runtime.execute` carrega `wamid` e `workflow`.
- `AfterTool` com `resultBytes = {"outcome":"replay"}` ⇒ o `ToolCallRecord` registrado tem `Outcome == "replay"`.
- `AfterTool` com `err != nil` ⇒ o `ToolCallRecord` é registrado com marcador de outcome de erro (ex. `"usecaseError"`), não some do sample.
- `internal/platform/scorer` não importa `internal/platform/agent` (layering verde); `Outcome` é `string` no kernel.
- Fixtures `*_test.go` do runtime/consumers atualizados com `MessageID` — suíte não quebra pela nova validação.
- Gates Go verdes no escopo alterado (build, vet, `test -race`, lint quando disponível) e gates de governança limpos (R-ADAPTER-001, DMMF, cardinalidade de métrica).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — primitivos Thread/Run/runtime, `RunSample`/`ScoringHooks` e layering scorer↔agent do substrato.
- `design-patterns-mandatory` — gate: sem novo GoF pattern (validação de fronteira + campo aditivo).

## Testes da Tarefa

- [ ] Testes unitários — `InboundRequest.Validate()`: `MessageID == ""` ⇒ erro nomeando `message_id` (`errors.Is(err, ErrEmptyMessageID)`); `MessageID` preenchido + demais campos válidos ⇒ `nil`. `extractOutcome`: `{"outcome":"replay"}` ⇒ `"replay"`; JSON sem `outcome` (read-tool) ⇒ `""`; JSON inválido/vazio ⇒ `""`. `AfterTool`: `resultBytes = {"outcome":"replay"}` ⇒ `ToolCallRecord.Outcome == "replay"`; `err != nil` ⇒ `ToolCallRecord` registrado com marcador de erro (não some).
- [ ] Testes de integração — se houver suíte de runtime/consumer que exercite `Execute`/`HandleInbound`, confirmar que run nasce com `correlation_key == WAMID` e que a validação rejeita inbound sem `MessageID`; varrer e corrigir fixtures que constroem `InboundRequest` sem `MessageID`.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/platform/agent/ports.go` — `InboundRequest.Validate()` (~L73-88), branch `MessageID == ""` (ADR-005 item 1).
- `internal/platform/agent/errors.go` — sentinela nova `ErrEmptyMessageID` (junto de `ErrEmptyAgentID`/`ErrEmptyMessage`).
- `internal/platform/agent/runtime.go` — span `agent.runtime.execute` (~L88), padronização `wamid`/`workflow` (ADR-005 item 5); `CorrelationKey: in.MessageID` (~L114) já existente.
- `internal/platform/scorer/scorer.go` — `ToolCallRecord` (~L10-14), campo aditivo `Outcome string` (ADR-004 item 1).
- `internal/agents/application/agents/scoring_hooks.go` — `AfterTool` (~L83), `extractOutcome` + parar de descartar `resultBytes` e `err` (ADR-004 item 2); consumido pela 4.0.
