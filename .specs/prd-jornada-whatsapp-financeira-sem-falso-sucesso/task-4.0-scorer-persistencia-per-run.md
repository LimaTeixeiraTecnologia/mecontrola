# Tarefa 4.0: Scorer de persistência per-run e diferenciação operacional

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Eliminar o falso sucesso per-run na avaliação de escrita: o scorer `tool-call-accuracy` atribuiu `1.0` ao primeiro `"Sim"` mesmo com ledger e transação vazios, porque só observa que uma tool foi invocada — nunca o efeito. Esta tarefa introduz um scorer code-based `write_persistence_accuracy` que reprova (0) qualquer marcador de sucesso sem efeito de persistência, endurece `no_hallucination` para exigir efeito real, e habilita a diferenciação operacional (RF-18) reusando os agregadores pós-deploy existentes. O sinal é per-run, determinístico, sem LLM e sem IO — o efeito é propagado como dado via `scorer.ToolCallRecord.Outcome` (já disponível pela tarefa 3.0). Detalhes em `techspec.md` e `adr-004-scorer-persistencia-per-run.md` — não duplicar aqui.

<requirements>
- RF-17, RF-18 conforme `prd.md`.
- Decisões firmes do ADR-004 (política falha-segura + endurecimento de `no_hallucination` + reuso dos agregadores; NÃO tocar `tool-call-accuracy`).
- DEPENDE da tarefa 3.0: consome `scorer.ToolCallRecord.Outcome` já propagado por `ScoringHooks.AfterTool` (campo aditivo, semântica fechada em `scorers`).
- `ScorerKind` permanece tipo fechado; novo scorer é `ScorerKindCodeBased`, SEM LLM.
- Nenhum IO/SQL no scorer — o efeito é dado, não consulta (ADR-004 alternativa (c) rejeitada).
- Layering preservado: `Outcome` é `string` no kernel; a semântica fechada dos valores vive em `internal/agents/application/scorers`.
- NÃO alterar `tool-call-accuracy` — baseline histórica 0,304 do ADR-004 do `prd-orquestracao-conversacional-confiavel` deve permanecer intacta.
- Sem novo GoF pattern — scorer code-based determinístico.
- Zero comentários em Go de produção (R-ADAPTER-001.1); manter comentários HTML de guard-rail deste arquivo.
</requirements>

## Subtarefas

- [ ] 4.1 (Novo scorer) Criar `internal/agents/application/scorers/write_persistence_accuracy.go` com `writePersistenceAccuracyScorer` (`ScorerKindCodeBased`, SEM LLM), reusando `mecontrolaWriteTools` de `behavioral_scorers.go` para identificar write-tools no `RunSample`. Política falha-segura por outcome: sem write-tool ⇒ neutro `1.0`; `routed`/`reconciled` ⇒ conta 1 (numerador e denominador); `replay` ⇒ ignorado (fora do denominador); `clarify` ⇒ ignorado; `usecaseError`/`missingResolver`/`truncated`/vazio-com-marcador ⇒ `0`; misto ⇒ `0` (qualquer write efetivada sem efeito puxa para `0`).
- [ ] 4.2 (Registro) Registrar `write_persistence_accuracy` em `BuildMeControlaScorers` (`mecontrola_scorers.go` ~L172-186) com `AlwaysSample()`.
- [ ] 4.3 (Endurecer `no_hallucination`) Em `behavioral_scorers.go` (~L195-225), o marcador de sucesso passa a exigir write-tool com `Outcome ∈ {routed, reconciled}`; `replay`/`clarify`/`usecaseError` não respaldam o sucesso.
- [ ] 4.4 (NÃO tocar `tool-call-accuracy`) Confirmar que nenhuma regra de `tool-call-accuracy` é alterada — baseline 0,304 intacta (ADR-004 do `prd-orquestracao-conversacional-confiavel`).
- [ ] 4.5 (RF-18 diferenciação operacional) Reusar os moldes de `internal/agents/infrastructure/persistence/postdeploy/aggregate_reader.go` — `RunAggregate` por outcome e `DuplicateWriteViolations` — parametrizando/trocando o `scorer_id` para `write_persistence_accuracy`. Sem query estrutural nova.
- [ ] 4.6 (Testes) Ver seção "Testes da Tarefa".

## Detalhes de Implementação

Ver `techspec.md` e `adr-004-scorer-persistencia-per-run.md` desta pasta — **referenciar em vez de duplicar**. ADR-004 fixa: (1) o campo `Outcome string` no `scorer.ToolCallRecord` e a razão do tipo `string` (evitar import `scorer → agent`, preservando layering — alternativa (d) rejeitada); (2) a política falha-segura completa do novo scorer (Decisão 3); (3) o contrato de endurecimento de `no_hallucination` (Decisão 4); (4) a proibição de alterar `tool-call-accuracy` (Decisão 5, R4); (5) o reuso de `aggregate_reader.go` para RF-18 sem query nova (Decisão 6). A dependência de propagação (`AfterTool` lendo `resultBytes`/`err` e gravando `Outcome`) é entregue pela tarefa 3.0 — esta tarefa CONSOME o campo já preenchido, não o repropaga.

## Critérios de Sucesso

- O scorer novo reprova (`0`) o primeiro `"Sim"` que invoca write-tool mas retorna `usecaseError` ou vazio-com-marcador (caso golden "primeiro Sim com ledger vazio" DEVE reprovar).
- `replay` e `clarify` são neutros — ficam fora do denominador (não penalizam idempotência/clarificação legítimas).
- Run sem write-tool ⇒ `1.0` (leitura não é penalizada).
- `routed`/`reconciled` ⇒ conta como efeito legítimo (score `1` quando único).
- Score misto (write efetivada sem efeito + write legítima no mesmo sample) ⇒ `0` (falha-segura).
- `no_hallucination` endurecido: marcador de sucesso com `replay`/`clarify`/`usecaseError` ⇒ `0`; marcador com `routed`/`reconciled` ⇒ `1`.
- `tool-call-accuracy` mantém o score do golden pré-mudança (não regressão da baseline 0,304).
- RF-18: `RunAggregate` por outcome e `DuplicateWriteViolations` disponíveis para `scorer_id = write_persistence_accuracy`.
- Gates Go verdes no escopo alterado (build, vet, `test -race`, lint quando disponível); gate de import (`scorer` não importa `agent`) verde; zero comentários em Go de produção.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — scorer-evals, `ScorerRunner`, `ScorerKind` fechado e `RunSample` do substrato.
- `design-patterns-mandatory` — gate: sem novo GoF pattern (novo scorer code-based determinístico).

## Testes da Tarefa

- [ ] Testes unitários — table-driven `RunSample → ScoreResult` para cada Outcome: `routed` ⇒ 1, `reconciled` ⇒ 1, `usecaseError` ⇒ 0, `missingResolver` ⇒ 0, `truncated` ⇒ 0, vazio-com-marcador ⇒ 0, `clarify` ⇒ neutro (fora do denominador), `replay` ⇒ neutro (fora do denominador), misto ⇒ 0, sem write-tool ⇒ `1.0`. Endurecimento de `no_hallucination`: marcador+`replay` ⇒ 0, marcador+`routed` ⇒ 1. Puros, sem IO/SQL. Confirmar que `tool-call-accuracy` mantém score do golden pré-mudança.
- [ ] Testes de integração — validação via golden na tarefa 8.0 (caso "primeiro Sim com ledger vazio" reprova; `write_persistence_accuracy` popula `platform_scorer_results` com `kind = code_based`).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/scorers/write_persistence_accuracy.go` — NOVO scorer code-based `writePersistenceAccuracyScorer` com a política falha-segura por outcome.
- `internal/agents/application/scorers/mecontrola_scorers.go` — registrar `write_persistence_accuracy` em `BuildMeControlaScorers` (~L172-186) com `AlwaysSample()`.
- `internal/agents/application/scorers/behavioral_scorers.go` — reuso de `mecontrolaWriteTools`; endurecer `no_hallucination` (~L195-225) exigindo `Outcome ∈ {routed, reconciled}`; NÃO tocar `tool-call-accuracy`.
- `internal/agents/infrastructure/persistence/postdeploy/aggregate_reader.go` — RF-18: `RunAggregate` por outcome + `DuplicateWriteViolations` parametrizados para `scorer_id = write_persistence_accuracy` (sem query estrutural nova).
