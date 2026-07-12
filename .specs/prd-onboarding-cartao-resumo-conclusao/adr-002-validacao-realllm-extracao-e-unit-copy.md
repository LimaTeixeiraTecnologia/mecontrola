# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Aceite dividido — exact-copy determinístico (unit) para copy e real-LLM para extração
- **Data:** 2026-07-12
- **Status:** Aceita
- **Decisores:** Time de plataforma (agents), solicitante do produto
- **Relacionados:** `.specs/prd-onboarding-cartao-resumo-conclusao/prd.md` (RF-18), `techspec.md`, `internal/agents/application/workflows/onboarding_workflow_integration_test.go`, `internal/agents/application/golden/`

## Contexto

O PRD (RF-18) exige prova real-LLM (`RUN_REAL_LLM=1`) cobrindo os comportamentos e testes determinísticos verdes, sem declarar nada pronto apenas com mock. Ao mapear o repositório, observou-se que:

- Os textos de cartão (palavra+emoji, "outro" em negrito, exemplo, estrutura do resumo) são **strings determinísticas emitidas pelo workflow**, não geradas por LLM. Para elas, teste exact-copy é prova mais forte que um julgamento por modelo.
- O harness golden (`internal/agents/application/golden`, `TestGoldenSetGate`) avalia o **agente** (loop de tool-calling com tools mockadas), não o workflow durável de onboarding; portanto não é o local para asserir a copy determinística do onboarding.
- Existe um harness real-LLM que dirige o **workflow de onboarding** ponta a ponta: `onboarding_workflow_integration_test.go` (`//go:build integration`, gate `RUN_REAL_LLM=1` + `OPENROUTER_API_KEY`, provider real, padrão `TestGoalValueCombinedExtractionGate`). A única parte dependente do modelo é a **extração** dos dados do cartão (ex.: "dia primeiro" → `dueDay=1`; banco sem apelido → `nickname=banco`).

## Decisão

A prova de aceite (RF-18) é composta por duas frentes complementares:

1. **Determinístico (unit, exact-copy):** em `onboarding_workflow_test.go`, asserts exatos sobre `cardsPrompt(0)`, `cardsPrompt(1)` (contendo `Deseja cadastrar **outro** cartão 💳 agora?`), cada `cardsRepromptFor(...)`, e `conclusionSummaryMessage` nos três desfechos (0, 1, ≥2 cartões). Atualizar o teste load-bearing `:1959`.
2. **Real-LLM (extração dependente do modelo):** novo `TestCardExtractionRealLLMGate` como método de suite em `OnboardingWorkflowRealLLMSuite` (`onboarding_workflow_integration_test.go`), executado via `-run 'TestOnboardingWorkflowRealLLMSuite/TestCardExtractionRealLLMGate'`, dirigindo `BuildCardsStep` com o agente real e um `CardManager` mockado que captura `CreateCard`, validando `"Nubank e vencimento dia primeiro"` → `DueDay==1`/`Nickname=="Nubank"`, `"Roxinho, Nubank e vencimento dia 1"` → `Nickname=="Roxinho"`, e `"Nubank e vencimento dia 1"` → `DueDay==1`. O gate roda com `AGENT_HARNESS_MODEL` (default `openai/gpt-4o-mini`), que é exatamente o modelo do agente/onboarding em produção — `AGENT_LLM_PRIMARY_MODEL=openai/gpt-4o-mini` (verificado em `.env.example:227`, `configs/config.go:183,1347,1047` e no container de produção). Não há modelo de onboarding separado; portanto o gate é prova fiel no mesmo modelo de produção, sem trade-off de fidelidade.

O gate golden do agente (`TestGoldenSetGate`, limiar `0.90`) permanece como não-regressão.

## Alternativas Consideradas

- **Cobrir tudo pelo harness golden do agente:** Inviável — o golden é agent-scoped e não exercita os prompts determinísticos do workflow de onboarding nem a extração interna do passo de cartões. Rejeitada.
- **Só testes determinísticos (mock) para tudo:** Viola RF-18 e a política de projeto (real-LLM obrigatório); não prova que o modelo interpreta "dia primeiro". Rejeitada.
- **Só real-LLM para tudo (incl. copy):** Frágil e caro; um juiz LLM sobre string fixa é mais fraco e não-determinístico que assert exato. Rejeitada.

## Consequências

### Benefícios Esperados

- Cada comportamento é provado pela técnica mais forte para sua natureza (exato para copy determinística; modelo real para extração).
- Zero falso positivo: a copy é verificada byte a byte; a extração é verificada com o modelo real.

### Trade-offs e Custos

- Duas suites a manter (unit + integration real-LLM). O gate real-LLM exige `OPENROUTER_API_KEY` e consome tokens.

### Riscos e Mitigações

- Risco: gate real-LLM flaky por variação do modelo. Mitigação: `Temperature: 0`, asserts sobre o resultado estruturado da extração (não sobre texto livre) e múltiplas frases-alvo; alinhado ao padrão já usado em `TestGoalValueCombinedExtractionGate`.
- Risco: divergência entre a copy do exemplo e o que o modelo aceita. Mitigação: o exemplo do prompt e as frases do gate real-LLM usam exatamente as mesmas formas ("dia 1"/"dia primeiro", com/sem apelido).

## Plano de Implementação

1. Escrever os asserts exact-copy (determinístico) junto com as mudanças de copy.
2. Adicionar `TestCardExtractionRealLLMGate` ao harness de integração do onboarding.
3. Rodar `go test ./...` (unit) e `RUN_REAL_LLM=1 OPENROUTER_API_KEY=... go test -tags integration -run TestCardExtractionRealLLMGate ./internal/agents/application/workflows -v`.
4. Rodar o gate golden do agente e confirmar ≥ 0.90 sem regressão.

## Monitoramento e Validação

- CI/execução local: ambos os gates verdes antes do merge.
- Pós-deploy: inspeção de conversa real (Postgres/`otel-lgtm`) confirmando extração correta de "dia primeiro" e copy nova.

## Impacto em Documentação e Operação

- Documentar no runbook de testes do agente o novo gate `TestCardExtractionRealLLMGate` e o comando de execução.

## Revisão Futura

- Revisitar se a copy do onboarding passar a ser gerada por LLM (deixaria de ser exact-copy) ou se o harness golden do agente passar a cobrir o workflow durável.
</content>
