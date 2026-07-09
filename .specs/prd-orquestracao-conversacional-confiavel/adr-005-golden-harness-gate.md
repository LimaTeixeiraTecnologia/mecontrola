# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Golden set versionado, harness em dois níveis e gate production-ready pós-deploy
- **Data:** 2026-07-09
- **Status:** Aceita
- **Decisores:** Plataforma / dono do agente MeControla
- **Relacionados:** `prd.md` (RF-35..RF-43, RF-49..RF-53), `techspec.md`, ADR-001..ADR-004, US-001

## Contexto

O PRD exige um golden set versionado cobrindo as intenções financeiras materiais e um gate
production-ready. Restrições e decisões do usuário:

- **Golden set:** sintéticos curados + incidentes reais **reescritos/anonimizados** (sem PII, sem
  WAMID/`resourceId`/`threadId` reais); nada verbatim de produção (RF-37, privacidade).
- **Harness em dois níveis:** CI por-PR **determinístico**; harness **real-LLM ≥ 0,90** sob
  tag/nightly + **pré-deploy** (custo/flakiness do OpenRouter não por-PR).
- **Gate em dois níveis:** pré-deploy bloqueante (golden + scorers + real-LLM ≥ 0,90) e pós-deploy
  monitorado com rollback, amostra mínima e margem; `tool-call-accuracy` redefinida.

Baseline produtiva (RF-49): 19 succeeded / 4 failed em 23 runs (7 dias); tool-call-accuracy 0,304,
completeness 0,149, categorization 0,565 — amostra pequena e métrica ruidosa.

## Decisão

1. **Golden set versionado** em `internal/agents/application/golden/` como fixtures Go. Cada caso
   declara `input`, `expectedTool`, `expectedArgs` (subconjunto), `expectedOutcome` e `responseProperty`
   verificável (RF-36). Composição: sintéticos + incidentes reescritos/anonimizados (RF-37). Cobre
   registro despesa/receita, C1–C7, cartões, orçamento/mês (named_without_year + verbatim + mês por
   extenso), recorrências, onboarding, pendências, confirmações, follow-up, erro de tool, ambiguidade,
   formato WhatsApp, ausência de termos internos (RF-35).
2. **Harness em dois níveis:**
   - **CI por-PR (determinístico):** testes unitários de cada guard e scorer + asserts do golden que são
     puros (scorers sobre `RunSample` fixo). Sem rede/LLM. Bloqueia o merge.
   - **Real-LLM (tag `realllm`, manual/nightly + pré-deploy):** dirige `BuildMeControlaAgent` com
     OpenRouter real sobre o golden, computando o **gate ≥ 0,90 por categoria de cenário**
     (cada categoria — registro, C1–C7, cartão, orçamento/mês, recorrência, onboarding, pendência,
     confirmação, follow-up, erro, ambiguidade, formato — deve bater ≥ 0,90 isoladamente, detectando
     regressão localizada; RF-39/40). Roda como gate **pré-deploy** bloqueante (RF-41).
3. **Gate pós-deploy (monitorado, com rollback):** consultas de agregação sobre `platform_runs`+
   `platform_scorer_results`+`workflow_runs`, exigindo **amostra mínima** (N ≥ 100 runs ou janela ≥ 14
   dias — o que ocorrer primeiro) e **margem absoluta por métrica** antes de promover/reverter, para
   separar melhora real de ruído (RF-43/51). Margens: taxa de falha ≤ baseline (4/23) **e** não maior
   que a baseline; scorers com ganho absoluto mínimo de **+0,05** sobre a baseline — `tool-call-accuracy`
   **redefinida** (denominador exclui `outcome ∈ {clarify, replay}`) ≥ 0,354; completeness ≥ 0,199;
   categorization ≥ 0,615; sem aumento de truncamento/escrita duplicada/resposta vazia/falha de
   persistência (RF-42/50/53). Decisão de promover/reverter é humana, rastreável por `run_id`, com
   evidência (RF-52).

## Alternativas Consideradas

- **Real-LLM em todo PR:** rejeitada pelo usuário — custo/flakiness/segredo por PR.
- **Só gate pré-deploy (sem rollback formal):** rejeitada — a US exige critério de reversão.
- **Canário pós-deploy sem gate local:** rejeitada — expõe usuários antes do gate.
- **Baselines literais sem amostra mínima/redefinição:** rejeitada — frágil estatisticamente
  (tool-call-accuracy 0,304 é ruído sobre 23 runs).
- **Prompts reais verbatim no golden:** rejeitada — conflita com a política de privacidade.

## Consequências

### Benefícios Esperados

- Regressão conversacional detectável por versão do agente (RF-38).
- Gate real antes de produção sem custo por-PR (RF-40).
- Decisão de rollback objetiva, com amostra mínima, evitando falso positivo sobre ruído (RF-51/52).
- Privacidade preservada no artefato versionado (RF-37).

### Trade-offs e Custos

- Manutenção do golden set (curadoria + anonimização de incidentes).
- Harness real-LLM exige `OPENROUTER_*` e tem variância — mitigada por gate estatístico ≥ 0,90 agregado,
  não por assert único frágil (lição registrada em revisões anteriores do projeto).

### Riscos e Mitigações

- **Risco:** brittleness de assert no harness real-LLM (falso-vermelho). **Mitigação:** invariante
  semântico/drive-until-state por cenário em vez de keyword estreita; gate por ratio agregado.
- **Risco:** amostra mínima atrasar decisão pós-deploy. **Mitigação:** janela ≤ 14 dias como teto;
  monitoração contínua com alertas antes do gate formal.
- **Rollback:** reverter a versão do agente e manter o golden como contrato de regressão.

## Plano de Implementação

1. Criar fixtures do golden (sintéticos + incidentes anonimizados).
2. Harness determinístico (CI) e real-LLM (tag `realllm`) com cálculo de ratio por categoria.
3. Queries de agregação do gate pós-deploy + limiares/margem no runbook.
4. Integrar o gate real-LLM ao passo pré-deploy.

Concluído quando: CI determinístico verde, real-LLM ≥ 0,90 pré-deploy, runbook do gate pós-deploy
publicado.

## Monitoramento e Validação

- Ratio do harness real-LLM por categoria (≥ 0,90).
- Agregados pós-deploy: falhas, scorers, truncamento, escrita duplicada, resposta vazia, persistência.
- Critério de revisão: se o gate bloquear indevidamente (falso positivo) ou deixar passar regressão,
  recalibrar margem/amostra.

## Impacto em Documentação e Operação

- Runbook: procedimento do gate pré e pós-deploy, queries e limiares.
- Dashboards: comparação por versão do agente; painel de rollback.
- Onboarding técnico: como adicionar caso ao golden e como rodar o harness real-LLM.

## Revisão Futura

Revisar limiares e amostra mínima após a primeira janela produtiva com a nova versão; ampliar o golden
conforme novos incidentes forem anonimizados.
