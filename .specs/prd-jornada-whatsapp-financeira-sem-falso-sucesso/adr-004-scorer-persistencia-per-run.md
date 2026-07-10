# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Scorer de persistência per-run via propagação de outcome ao RunSample
- **Data:** 2026-07-10
- **Status:** Aceita
- **Decisores:** time de plataforma / agente financeiro / qualidade
- **Relacionados:** PRD `prd-jornada-whatsapp-financeira-sem-falso-sucesso` (RF-17, RF-18); `techspec.md`; ADR-004 do `prd-orquestracao-conversacional-confiavel` (baseline `tool-call-accuracy`); US-001

## Contexto

O scorer `tool-call-accuracy` atribuiu score `1.0` ao primeiro `"Sim"` da jornada mesmo com ledger e transação vazios: um falso sucesso per-run. A causa é uma cadeia de descarte de dados que impede qualquer scorer de enxergar o efeito real da tool de escrita.

- `internal/platform/scorer/scorer.go`: `ToolCallRecord{ID,Name,Args}` (L10-14) não possui campo de outcome; `RunSample{Input,Output,ExpectedOutput,ToolCalls,Metadata}` (L16-22) carrega apenas nome e args das tools.
- `internal/agents/application/agents/scoring_hooks.go`: `AfterTool(ctx,_,toolID,argsJSON,_ []byte,err error)` (L83) descarta o 4º parâmetro `resultBytes` — o JSON de saída da tool, que contém `outcome` e `resourceId`. Além disso, `if err != nil { return }` (L84) descarta toda tool que retornou erro Go, fazendo um `usecaseError` desaparecer do sample. `AfterExecute` (L59-63) deixa `Metadata` nil em produção.
- Scorers `tool-call-accuracy` e `no_hallucination` (`internal/agents/application/scorers/behavioral_scorers.go`, L195-225) só observam o NOME da tool invocada — nunca o efeito.
- A verdade do efeito já existe no fluxo, apenas não é propagada ao sample: `internal/agents/application/usecases/idempotent_write.go` produz `IdempotentWriteResult{ResourceID, Outcome agent.ToolOutcome}` → `RegisterResult.Outcome` → `tools/register_expense.go` emite JSON com `outcome` (required no schema strict) e `resource` vazio quando `clarify`.
- Restrições: `ScorerKind` é tipo fechado (`code-based` | `llm-judged`) em `internal/platform/scorer/types.go`; o kernel scorer não pode importar `internal/platform/agent` (layering); o scorer não pode fazer IO (R-WF-KERNEL / R-ADAPTER).

RF-17 exige um sinal per-run que reprove qualquer marcador de sucesso sem efeito de persistência. RF-18 exige diferenciação operacional pós-deploy sem introduzir consulta estrutural nova.

## Decisão

1. **Propagar o efeito como DADO (sem IO no scorer).** Adicionar campo `Outcome string` a `scorer.ToolCallRecord`. O tipo é `string`, não `agent.ToolOutcome`, para NÃO importar `internal/platform/agent` no kernel scorer — preserva o layering. A semântica fechada dos valores (`routed`, `reconciled`, `replay`, `clarify`, `usecaseError`, `missingResolver`, `truncated`) vive no pacote de domínio `internal/agents/application/scorers`, que já conhece `agent`.

2. **`ScoringHooks.AfterTool` para de descartar dados.** Passa a ler `resultBytes` via `extractOutcome(resultBytes)`, que desserializa `{"outcome":string}` de forma genérica (`""` para read-tools que não têm o campo) e grava em `ToolCallRecord.Outcome`. E para de descartar tools com erro Go: quando o `exec` retorna erro, a tool é registrada com um marcador de outcome de erro (ex.: `"usecaseError"`) para o scorer enxergar a falha.

3. **Novo scorer code-based `write_persistence_accuracy`** (`ScorerKindCodeBased`, SEM LLM). Política de score, falha-segura:
   - sem write-tool no sample ⇒ neutro `1.0`;
   - `routed` / `reconciled` ⇒ conta como efeito legítimo (1 no numerador e denominador);
   - `replay` ⇒ ignorado (idempotência legítima, fora do denominador);
   - `clarify` ⇒ ignorado (não é escrita efetivada);
   - `usecaseError` / `missingResolver` / `truncated` / vazio-com-marcador ⇒ `0` (qualquer write que se efetivou sem efeito reprova).

4. **Endurecer `no_hallucination`.** O marcador de sucesso passa a exigir uma write-tool com `Outcome ∈ {routed, reconciled}` — não basta a write-tool ter sido invocada, e `replay` / `clarify` / `usecaseError` não respaldam o sucesso.

5. **NÃO tocar em `tool-call-accuracy`.** O ADR-004 do `prd-orquestracao-conversacional-confiavel` exige continuidade da baseline histórica (0,304) e do gate pós-deploy. O novo scorer é isolado; nenhuma regra de `tool-call-accuracy` é alterada.

6. **RF-18 reusa moldes existentes.** A diferenciação operacional reaproveita `internal/agents/infrastructure/persistence/postdeploy/aggregate_reader.go`: `RunAggregate` por outcome e `DuplicateWriteViolations` apenas trocando o `scorer_id` para `write_persistence_accuracy`. Sem query estrutural nova.

Partes impactadas: kernel scorer (campo aditivo), hooks de scoring do agente, pacote `scorers` (novo scorer + endurecimento), agregação pós-deploy (parametrização de `scorer_id`).

## Alternativas Consideradas

- **(a) Redefinir `tool-call-accuracy` para exigir efeito.** Vantagem: um único scorer. Desvantagem: quebra a baseline histórica (0,304) e invalida o gate pós-deploy do ADR-004 anterior. Rejeitada.
- **(b) Só reforçar a agregação pós-deploy.** Vantagem: nenhuma mudança no caminho quente. Desvantagem: mantém o falso positivo per-run — o `"Sim"` com ledger vazio continuaria pontuando `1.0`. Rejeitada.
- **(c) Scorer faz SQL/lookup no ledger para confirmar persistência.** Vantagem: verdade de banco. Desvantagem: IO no scorer viola R-WF-KERNEL / R-ADAPTER; o efeito deve ser propagado como dado, não consultado. Rejeitada.
- **(d) `Outcome agent.ToolOutcome` no kernel scorer.** Vantagem: tipo fechado direto. Desvantagem: cria import `scorer → agent`, invertendo o layering. Rejeitada; a semântica fechada fica no pacote `scorers`.

## Consequências

### Benefícios Esperados

- Elimina o falso sucesso per-run: um marcador de sucesso sem efeito de persistência passa a pontuar `0`.
- Sinal de qualidade determinístico, sem LLM e sem IO, verificável por teste unitário.
- Layering preservado: kernel scorer permanece agnóstico ao domínio.
- Baseline e gate do ADR-004 anterior intactos; o novo sinal é aditivo.
- RF-18 sem custo estrutural — reusa agregadores existentes.

### Trade-offs e Custos

- `RunSample`/`ToolCallRecord` ganham um campo; toda produção de sample precisa preenchê-lo (default `""`).
- `AfterTool` passa a desserializar `resultBytes` em todo run (custo marginal de um `json.Unmarshal` genérico).
- Um novo `scorer_id` a operar em `platform_scorer_results`.

### Riscos e Mitigações

- **R1 — mudar `RunSample`/`ToolCallRecord`:** aditivo e contido; campo novo com zero-value `""` não quebra scorers existentes.
- **R2 — `AfterTool` cego:** obrigatório parar de descartar TANTO `resultBytes` QUANTO o `err`; se apenas um for tratado, o scorer permanece cego a parte das falhas. Mitigação: teste que cobre write com erro Go e write com outcome de erro no JSON.
- **R3 — layering:** `Outcome` é `string` no kernel; a semântica fechada mora em `scorers`. Gate de import (`scorer` não importa `agent`) permanece verde.
- **R4 — baseline:** proibido alterar `tool-call-accuracy`; validar que seu score continua idêntico ao golden pré-mudança.
- **R5 — read-tools sem outcome:** `extractOutcome` retorna `""`; sem write-tool ⇒ scorer neutro `1.0` (não penaliza leitura).
- **R6 — score misto:** política falha-segura — qualquer write efetivada sem efeito puxa para `0`; `replay`/`clarify` ficam fora do denominador para não penalizar idempotência/clarificação legítimas.

Rollback: remover o registro do `write_persistence_accuracy` do runner e reverter o endurecimento de `no_hallucination`; o campo `Outcome` pode permanecer inerte (zero-value) sem efeito colateral.

## Plano de Implementação

1. Adicionar `Outcome string` a `scorer.ToolCallRecord` (`internal/platform/scorer/scorer.go`).
2. Em `scoring_hooks.go`: implementar `extractOutcome(resultBytes)`; gravar `Outcome` no `ToolCallRecord`; registrar tools com erro Go usando marcador de outcome de erro (remover o `return` incondicional em `err != nil`).
3. Criar `writePersistenceAccuracyScorer` (`ScorerKindCodeBased`) em `internal/agents/application/scorers/`, com a tabela de política falha-segura; registrar no runner.
4. Endurecer `noHallucinationScorer`: exigir write-tool com `Outcome ∈ {routed, reconciled}`.
5. RF-18: parametrizar `aggregate_reader.go` para `scorer_id = write_persistence_accuracy` (`RunAggregate` + `DuplicateWriteViolations`).
6. Testes unitários table-driven `RunSample → ScoreResult` por outcome; caso golden do "primeiro Sim com ledger vazio".

Dependências: fluxo de `idempotent_write.go` / `register_expense.go` já emite `outcome` no JSON (pré-existente). Sequência: 1 → 2 → 3 ‖ 4 → 5 → 6.

Adoção concluída quando: novo scorer registrado e populando `platform_scorer_results`; caso golden reprova; baseline de `tool-call-accuracy` inalterada.

## Monitoramento e Validação

- Novo scorer grava em `platform_scorer_results` com `scorer_id = write_persistence_accuracy` e `kind = code_based`.
- Teste unitário table-driven cobrindo cada outcome (`routed`/`reconciled` ⇒ 1; `replay`/`clarify` ⇒ neutro; `usecaseError`/`missingResolver`/`truncated`/vazio-com-marcador ⇒ 0; sem write-tool ⇒ 1).
- Caso golden "primeiro Sim com ledger vazio" DEVE reprovar (`0`).
- Validar que `tool-call-accuracy` mantém o score do golden pré-mudança (não regressão da baseline 0,304).
- RF-18: `RunAggregate` por outcome disponível no reader pós-deploy.
- Critério para revisão: divergência entre `write_persistence_accuracy` per-run e a agregação pós-deploy, ou surgimento de novo `agent.ToolOutcome` não mapeado na política.

## Impacto em Documentação e Operação

- `techspec.md`: registrar o novo scorer e o contrato de `Outcome` no `RunSample`.
- Runbook do agente / dashboards de scorers: incluir `write_persistence_accuracy`.
- Governança: confirmar gate de import (`scorer` não importa `agent`) e zero comentários em Go.

## Revisão Futura

- Revisar quando um novo `agent.ToolOutcome` for introduzido (mapear na política falha-segura).
- Revisar se a distância entre sinal per-run e agregação pós-deploy indicar drift.
- Substituir por nova ADR se `tool-call-accuracy` for redefinido no `prd-orquestracao-conversacional-confiavel`.

## Conformidade

- SEM novo GoF pattern: apenas campo aditivo em `ToolCallRecord` + novo scorer code-based determinístico.
- `ScorerKind` permanece tipo fechado; novo scorer é `ScorerKindCodeBased`.
- Nenhum LLM no scorer novo; nenhum IO no scorer (efeito propagado como dado).
- Layering preservado (`Outcome string` no kernel; semântica fechada em `scorers`).
- Zero comentários em Go.
