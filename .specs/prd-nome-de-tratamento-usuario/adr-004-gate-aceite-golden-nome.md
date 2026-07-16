# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Gate de aceite golden real-LLM (≥ 0,90 por categoria, 0 falso-sucesso) com categoria `CategoryTreatmentName`
- **Data:** 2026-07-15
- **Status:** Aceita
- **Decisores:** Autor do PRD/techspec, confirmado pelo solicitante (múltipla escolha)
- **Relacionados:** `prd.md` (RF-02, RF-05, RF-12, RF-14, RF-16), `techspec.md`, `.claude/rules/go-testing.md`, ADR-002

## Contexto

- RF-14 exige gate de aceite real-LLM com score ≥ 0,90 e 0 falso-sucesso; RF-05/RF-12 exigem uso natural do nome e aderência ao Tom de Voz.
- Harness existente: `harness_realllm_test.go` roda sob `//go:build integration` + `RUN_REAL_LLM=1` + `OPENROUTER_API_KEY`, com `goldenGateThreshold = 0.90` (`:24`), 3 repetições/caso (`:390`), gate **por categoria** (`TestGoldenSetGate`, `:392-426`). Casos são coletados por append em `AllCases()` (`registry.go:8-26`); `Category` é tipo fechado (`case.go:7-70`) com lista `required` em `registry_test.go:28-48`. Tools do harness vêm de `goldenToolCatalog` (`:248`).
- 0 falso-sucesso é garantido por scorers (`no_hallucination`, `write_persistence_accuracy` em `behavioral_scorers.go`) + invariantes unit estilo `journey_test.go`. Tom: `tone_adherence` (LLM-judged, `mecontrola_scorers.go:181-183`) + `verbatim_tone_adherence` (determinístico: asterisco simples + emoji oficial, `:307-346`).

## Decisão

Adotar o gate golden existente para o fluxo, adicionando uma categoria fechada `CategoryTreatmentName` e casos dedicados, reutilizando os scorers de tom e de não-alucinação (sem novos scorers):

- `case.go`: novo const `CategoryTreatmentName`, estendendo `IsValid()`/`AllCategories()`; `registry_test.go`: incluir na lista `required`.
- `cases_treatment_name.go` (append em `AllCases()`): casos "alterar com nome" (`ExpectedTool: edit_treatment_name`, `ExpectedArgs: {name}`), "alterar sem nome" (`ExpectedTool: edit_treatment_name` sem arg) e "confirmação no tom" (`ResponseProperty` de asterisco simples + emoji oficial via `requires_brand_emoji`).
- `goldenToolCatalog`: stub `edit_treatment_name` (capture tool).
- Invariante de onboarding (composição de working memory preservando o sentinel) validada por unit no estilo `journey_test.go` (pura, sem LLM).
- Extração do nome (RF-02) validada por `DecideTreatmentName` (suíte) + casos golden que exercem a seleção de tool com/sem nome.

## Alternativas Consideradas

- **Só testes determinísticos (unit + scorers determinísticos), sem gate real-LLM.** Vantagem: barato/rápido. Desvantagem: não valida extração NL nem uso natural do nome com o LLM real; viola RF-14. Rejeitada.
- **Reutilizar `CategoryGoal` em vez de criar categoria nova.** Desvantagem: mistura métricas de gate de fluxos distintos; o gate é por categoria, então casos de nome diluiriam/contaminariam o ratio do goal. Rejeitada — categoria dedicada dá sinal limpo.
- **Novo scorer determinístico de "uso do nome".** Desvantagem: difícil codificar "uso natural sem excesso" de forma robusta; risco de falso-negativo. Rejeitada — coberto pelo `tone_adherence` LLM-judged.

## Consequências

### Benefícios Esperados

- Sinal de qualidade limpo e isolado para o fluxo; 0 falso-sucesso herdado de guardas já existentes.
- Reuso do harness/scorers, custo incremental baixo.

### Trade-offs e Custos

- Execução do gate exige credenciais OpenRouter e é mais lenta (3× por caso). Roda em CI/integration, não no unit.
- Manutenção da lista `required` de categorias (uma linha).

### Riscos e Mitigações

- Risco: flutuação do LLM abaixo de 0,90. Mitigação: instrução-por-exemplo nos casos e nas instruções do agente; 3 repetições absorvem ruído; ajustar copy/instrução se necessário (sem baixar o threshold).
- Rollback: remover casos/categoria não afeta produção (é teste).

## Plano de Implementação

1. Categoria + `IsValid`/`AllCategories` + `required`.
2. Builder de casos + append em `AllCases()`.
3. Stub de tool no catálogo.
4. Invariante unit de onboarding.
5. Rodar `RUN_REAL_LLM=1 ... go test -tags integration ./internal/agents/application/golden/ -run TestGoldenSetGate`.

## Monitoramento e Validação

- Resultado do gate por categoria em CI; scorers de tom no relatório.
- Critério de sucesso: `CategoryTreatmentName` passa ≥ 0,90 nas 3 repetições; invariantes unit verdes.

## Impacto em Documentação e Operação

- Documentar a nova categoria e como rodar o gate no README de testes de `internal/agents`.

## Revisão Futura

- Revisitar o threshold/estratégia se o fluxo ganhar mais variações de linguagem ou se o modelo padrão mudar.
