# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Scorer de tool esperada por cenário + harness de validação com LLM real
- **Data:** 2026-07-02
- **Status:** Aceita
- **Decisores:** Autor da techspec, time de plataforma
- **Relacionados:** PRD (RF-21, RF-29, RF-30, RF-33, M-04, M-05); techspec; ADR-005; memória
  `feedback_realllm_validation_required`

## Emenda spec-version 3 (2026-07-02) — assert de linhas reais no banco para cenários de escrita

À luz da evidência de produção (PRD, `Evidência de Produção`, EP-01/EP-05: sucesso alucinado com 0
linhas gravadas), o harness real-LLM passa a **exigir assert de linhas reais no banco** para todo
cenário de escrita (RF-29/RF-33/M-05, D-10). Nem o Run marcar `succeeded`, nem o scorer indicar tool
chamada, contam como prova de escrita: cada cenário de escrita DEVE verificar a existência das linhas
correspondentes em `transactions`, `transactions_card_purchases`, `agents_write_ledger` e
`transactions_recurring_templates`. O conjunto canônico do harness cobre os cenários da evidência de
produção **EP-01..EP-05** (escrita perdida, leitura de orçamento inoperante, listar categorias, atrito
de confirmação e Run que não discrimina), servindo de regressão determinística para a correção de
substrato da ADR-005.

## Contexto

O PRD exige uso **efetivo** e **determinístico** das tools: cada tool registrada deve ser exercida em
execução real (RF-29) e a seleção de tool deve acertar ≥ 0.90 num conjunto canônico (M-04, RF-21). O
scorer atual `tool-call-accuracy` (`anyFinancialToolScorer`, `mecontrola_scorers.go:37-73`) pontua 1.0
se **qualquer** tool financeira for chamada — insuficiente para provar que a tool **correta** foi
escolhida, e cego a tools não exercidas. Mocks não são evidência aceitável para fixes do agent
(memória).

## Decisão

Introduzir um scorer de **tool esperada por cenário**: dado um cenário canônico com a tool esperada,
pontua 1.0 apenas se a tool chamada for a esperada. Criar um harness E2E `*_realllm_test.go` gated por
`RUN_REAL_LLM=1` + `OPENROUTER_*` do `.env`, com um conjunto determinístico cobrindo **todas** as 24
tools (9 + 15). O harness mede M-04 (≥ 0.90) e garante RF-29 (cada tool exercida ≥ 1 vez). Se
necessário para o expected-match, expor `ExpectedTool`/`Args` em `scorer.RunSample`/`ToolCallRecord`
(`internal/platform/scorer`), mantendo tipos fechados e cardinalidade controlada.

## Alternativas Consideradas

- **Manter o scorer coarse.** Vantagem: zero mudança. Desvantagem: permite falso positivo de cobertura
  (RF-30) e não mede seleção correta. Rejeitada.
- **Só testes unitários com mock do LLM.** Desvantagem: não prova seleção real; contraria a memória de
  validação com LLM real. Rejeitada como evidência única (permanece como teste complementar).
- **Avaliação humana ad hoc.** Desvantagem: não reprodutível, não versionável. Rejeitada.

## Consequências

### Benefícios Esperados

- Mede seleção correta (não só "alguma tool"), fechando RF-21/RF-29/M-04.
- Bloqueia falso positivo de cobertura via gate objetivo (RF-30).
- Conjunto canônico versionado vira regressão determinística.

### Trade-offs e Custos

- Custo de tokens do harness real-LLM (execução gated, não em CI padrão).
- Manutenção do conjunto canônico ao adicionar/renomear tools.

### Riscos e Mitigações

- **Risco:** flutuação do modelo derruba M-04. **Mitigação:** barra 0.90 (não 1.0); cenários não
  ambíguos; descrições de tool precisas.
- **Rollback:** o scorer novo é aditivo; pode coexistir com o coarse durante transição.

## Plano de Implementação

1. Implementar `NewExpectedToolScorer` (code-based) e, se preciso, `ExpectedTool` em `RunSample`.
2. Montar o conjunto canônico (1 tool esperada por cenário) cobrindo as 24 tools.
3. Escrever `*_realllm_test.go` gated por `RUN_REAL_LLM`.
4. Atualizar `BuildMeControlaScorers` e a lista `mecontrolaFinancialTools`.

## Monitoramento e Validação

- Score de expected-tool por run em dashboard; alerta se M-04 < 0.90.
- Relatório do harness lista tools não exercidas (deve ser vazio para RF-29).

## Impacto em Documentação e Operação

- Documentar como rodar o harness (`RUN_REAL_LLM=1`, credenciais) no runbook do agente.

## Revisão Futura

- Reavaliar a barra 0.90 após dados reais; considerar validação de argumentos da tool (não só o nome).
