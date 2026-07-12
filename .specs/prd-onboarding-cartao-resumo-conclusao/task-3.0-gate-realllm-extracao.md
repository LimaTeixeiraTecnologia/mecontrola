# Tarefa 3.0: Gate real-LLM da extração de cartão (dia primeiro, banco sem apelido)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Provar, com o modelo real, que o exemplo prometido ao usuário funciona de ponta a ponta: a extração de cartão aceita o dia por extenso ("dia primeiro") e o banco sem apelido (herança apelido←banco). Adicionar `TestCardExtractionRealLLMGate` ao harness de integração já existente do onboarding e confirmar que o gate golden do agente não regride.

<requirements>
- RF-07 (lado aceitação): o sistema aceita o dia em ambos os formatos ("dia 1" e "dia primeiro").
- RF-09 (lado extração): informar apenas o banco cria cartão com `nickname == banco`.
- RF-18: eval golden real-LLM (`RUN_REAL_LLM=1`) cobrindo os comportamentos dependentes do modelo, sem declarar pronto apenas com mock; gate golden do agente permanece verde.
</requirements>

## Subtarefas

- [x] 3.1 Adicionar `TestCardExtractionRealLLMGate` como **método de suite** em `OnboardingWorkflowRealLLMSuite` (`onboarding_workflow_integration_test.go`, `//go:build integration`, gate `RUN_REAL_LLM=1` + `OPENROUTER_API_KEY`), seguindo o padrão do método existente `TestCardExtractionGate` (`:553`) — que hoje só valida `wantsCard`. O provider real vem do `SetupTest` da suite; por isso DEVE ser método de suite (não `func` top-level solto sem provider). Dirigir `BuildCardsStep(a, cards)` com o agente real e um `CardManager` mockado que **captura os argumentos de `CreateCard`** (não usar `.Maybe()` sem captura) para asserir `DueDay`/`Nickname`.
- [x] 3.2 Cenários: `"Nubank e vencimento dia primeiro"` → `DueDay==1`/`Nickname=="Nubank"`; `"Roxinho, Nubank e vencimento dia 1"` → `Nickname=="Roxinho"`/`Bank=="Nubank"`/`DueDay==1`; `"Nubank e vencimento dia 1"` → `DueDay==1`/`Nickname=="Nubank"`.
- [x] 3.3 Comando de aceite DEVE usar o caminho de subteste da suite (senão `-run` não casa e passa vazio/falso-positivo): `RUN_REAL_LLM=1 OPENROUTER_API_KEY=... go test -tags integration -run 'TestOnboardingWorkflowRealLLMSuite/TestCardExtractionRealLLMGate' ./internal/agents/application/workflows -v`.
- [x] 3.4 Confirmar que o gate golden do agente (`internal/agents/application/golden`, `TestGoldenSetGate`, limiar 0.90) permanece verde (não introduzir regressão de categoria).

## Detalhes de Implementação

Ver `techspec.md` seção "Testes de Integração / Real-LLM (RF-18)" e ADR-002. O gate roda com `AGENT_HARNESS_MODEL` (default `openai/gpt-4o-mini`), que é exatamente o modelo de produção `AGENT_LLM_PRIMARY_MODEL=openai/gpt-4o-mini` (verificado em config e no container de produção) — prova fiel. Asserts sobre o resultado estruturado da extração (args de `CreateCard`), não sobre texto livre; `Temperature: 0`.

## Critérios de Sucesso

- `RUN_REAL_LLM=1 OPENROUTER_API_KEY=... go test -tags integration -run 'TestOnboardingWorkflowRealLLMSuite/TestCardExtractionRealLLMGate' ./internal/agents/application/workflows -v` verde para os três cenários (RF-07/RF-09), com o subteste efetivamente executado (não zero testes).
- Gate golden do agente permanece ≥ 0.90 sem regressão (RF-18).
- Nenhuma alteração no schema de extração (`cardSchema`/`cardsSystemPrompt`) — o gate valida o comportamento existente sob as novas formas de exemplo.

## Skills Necessárias

<!-- MANDATÓRIO: go-implementation é auto-carregada por detecção de diff (category: language). -->

- `mastra` — o teste exercita a call-site sancionada de LLM (agente real) no passo de cartões; seguir o padrão de harness real-LLM do consumidor.
- `domain-modeling-production` — validar que a extração produz o comando de cartão correto (herança apelido←banco) como invariante de domínio.
- `design-patterns-mandatory` — confirmar que a prova não introduz abstração de teste desnecessária (reusar o harness existente).

## Testes da Tarefa

- [x] Testes unitários (não aplicável; comportamento determinístico coberto na 1.0/2.0)
- [x] Testes de integração (`TestCardExtractionRealLLMGate` real-LLM; gate golden do agente)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/onboarding_workflow_integration_test.go` — novo `TestCardExtractionRealLLMGate`.
- `internal/agents/application/golden/` — gate do agente (verificar não-regressão; sem novos casos obrigatórios).
</content>
