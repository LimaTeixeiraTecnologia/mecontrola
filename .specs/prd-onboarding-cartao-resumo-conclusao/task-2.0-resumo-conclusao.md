# Tarefa 2.0: Resumo + conclusão do onboarding no passo de conclusão

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Enriquecer o passo de conclusão para emitir, ao encerrar a etapa de cartões (com ou sem cartão, um ou vários), um "Resumo de Onboarding" completo seguido da frase de conclusão existente. Implementar `conclusionSummaryMessage` (função pura), ampliar `BuildConclusionStep` para receber `BudgetPlanner` e `CardManager` (já disponíveis no wiring) e encaminhá-los em `BuildOnboardingWorkflow`.

<requirements>
- RF-10: ao encerrar a etapa de cartões, apresentar "Resumo de Onboarding" + conclusão e encerrar.
- RF-11: resumo apresentado uma única vez, ao final, nunca a cada cartão.
- RF-12: resumo contém objetivo, valor da meta (se houver), orçamento mensal, distribuição por categoria, cartões (ou "nenhum cartão 💳"), e estado da recorrência.
- RF-13: seção usa o título "Resumo de Onboarding" (recebe 📊 automático do normalizador).
- RF-14: valores refletem exatamente o estado persistido; distribuição via `SuggestAllocation`, cartões via `ListCards`.
- RF-15: categorias com rótulos+emoji padronizados; valores em BRL na formatação existente.
- RF-16: sem cartão, indicar explicitamente "nenhum cartão 💳 cadastrado".
- RF-17: sem regressão — só copy/montagem; sem mudança em ordem de etapas, suspend/resume, criação de cartão, ativação, recorrência ou idempotência.
</requirements>

## Subtarefas

- [ ] 2.1 Implementar `conclusionSummaryMessage(state, items []interfaces.AllocationCents, cards []interfaces.Card) string` (pura) conforme a estrutura exata da techspec, reutilizando `renderAllocationLines`, `categoryLabels`, `money.FromCents(...).BRL()` e a cauda de `conclusionFinalMessage` (inalterado).
- [ ] 2.2 Ampliar `BuildConclusionStep(workingMem, budgets interfaces.BudgetPlanner, cards interfaces.CardManager)` (`:1036`): após o upsert de WM, chamar `SuggestAllocation(state.MonthlyBudgetCents, allocationBPList(state.Allocations))` e `ListCards(userUUID)`, compor `state.FinalMessage`.
- [ ] 2.3 Encaminhar `budgets` e `cards` a `BuildConclusionStep` no registro do `stepConclusionID` em `BuildOnboardingWorkflow` (`:1110`).
- [ ] 2.4 Tratamento de erro: falha de `SuggestAllocation`/`ListCards` → `failStep` com erro embrulhado (consistente com o `WorkingMemory.Upsert` atual); sem degradação silenciosa (RF-14).
- [ ] 2.5 Formato da linha de cartão: `- <Bank> — vencimento dia <DueDay>` quando `Nickname == Bank`; `- <Nickname> (<Bank>) — vencimento dia <DueDay>` caso contrário. Sem cartões → `Nenhum cartão 💳 cadastrado.`
- [ ] 2.6 Migrar TODAS as 5 chamadas existentes de `BuildConclusionStep(s.wmMock)` para a nova assinatura de 3 args em `onboarding_workflow_test.go:1980,2006,2027,2050,2070` (a quebra é de compilação do binário de teste whitebox inteiro, que não tem build tag). Adicionar as expectativas de mock de `SuggestAllocation` e `ListCards` em cada teste — em especial `TestBuildConclusionStep_DoesNotReopenDistributionSummaryOrActivation` (`:2050`), que hoje só configura `s.wmMock` e passaria a invocar `budgets`/`cards` não-mockados.
- [ ] 2.7 Testes unitários dos três desfechos (0, 1, ≥2 cartões) com `BudgetPlanner`/`CardManager` mockados; manter verdes os testes de `FinalMessage` existentes (`:1987,2011`) e o exact-copy de `conclusionFinalMessage` (`:2083-2092`).

## Detalhes de Implementação

Ver `techspec.md` seções "Copy — especificação exata" (bloco do resumo), "Interfaces Chave", "Modelos de Dados", "Tratamento de erros" e ADR-001 (fontes de verdade) e ADR-003 (não aplicar padrão). Não alterar `module.go`, kernel, schemas de extração nem `conclusionFinalMessage`.

## Critérios de Sucesso

- Resumo emitido uma única vez ao final, cobrindo os três desfechos (sem/1/≥2 cartões) com conteúdo completo (RF-10..RF-16).
- Distribuição byte-idêntica à revisão pré-ativação (mesma fonte `SuggestAllocation`) e título "Resumo de Onboarding" recebendo 📊 pós-normalização (RF-13/RF-14).
- Falha de IO na conclusão falha o passo (retry por `MaxAttempts:3`), sem resumo parcial (RF-14).
- As 5 chamadas existentes de `BuildConclusionStep` (`:1980,2006,2027,2050,2070`) compilam e passam com a nova assinatura de 3 args + mocks de `SuggestAllocation`/`ListCards`; o binário de teste whitebox `workflows` compila limpo.
- Sem mudança em `module.go`, kernel, ordem de etapas ou idempotência (RF-17); zero comentários no Go alterado (R-ADAPTER-001.1).

## Skills Necessárias

<!-- MANDATÓRIO: go-implementation é auto-carregada por detecção de diff (category: language). -->

- `mastra` — altera passo de workflow durável (`BuildConclusionStep`) consumindo o substrato; preservar suspend/resume e Run auditável.
- `domain-modeling-production` — o resumo é projeção de estado/fontes de verdade; garantir funções puras e nenhum estado de domínio novo (DMMF).
- `design-patterns-mandatory` — confirmar "não aplicar padrão" para o montador de resumo (função pura, sem Builder/Strategy).

## Testes da Tarefa

- [ ] Testes unitários (três desfechos do resumo; não-regressão dos testes de `FinalMessage` e `conclusionFinalMessage`)
- [ ] Testes de integração (não aplicável; extração real-LLM na 3.0)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/onboarding_workflow.go` — `conclusionSummaryMessage`, `BuildConclusionStep`, `BuildOnboardingWorkflow`.
- `internal/agents/application/workflows/onboarding_workflow_test.go` — testes do passo de conclusão.
- `internal/agents/application/interfaces/{card_manager.go,budget_planner.go,types.go}` — lidos (sem alteração).
</content>
