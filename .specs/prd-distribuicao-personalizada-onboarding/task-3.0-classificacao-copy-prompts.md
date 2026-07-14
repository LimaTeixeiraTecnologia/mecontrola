# Tarefa 3.0: Classificação de intenção onboarding-only, copy e prompts

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Adicionar a classificação de intenção onboarding-only (`accept | personalize | values` + `mixed_unit`) que roda antes da extração de valores compartilhada, e a camada de renderização de mensagens: prompt de personalizar (âncora + ZERO), copy do prompt de distribuição (anúncio), mensagem de saldo e aviso de categoria zerada no resumo.

<requirements>
- RF-01: recusa/intenção de personalizar abre modo personalizar (pergunta por categoria, reforça distribuir tudo, explica ZERO).
- RF-02: prompt de distribuição anuncia a opção "não → personalizar", mantendo "Aceita esta sugestão".
- RF-03: prompt de personalizar mostra o orçamento mensal como âncora e as 5 categorias com rótulos.
- RF-07: categoria zerada aceita com aviso único anexado ao resumo (onboarding-only).
- RF-10: unidades misturadas geram pedido de unidade única.
</requirements>

## Subtarefas

- [ ] 3.1 Criar `distributionIntentSchema` (apenas `action` + `mixed_unit`, `additionalProperties:false`), `distributionIntentSystemPrompt` (com exemplos por extenso e precedência `values` > `personalize` > `accept`) e `distributionIntentExtract`.
- [ ] 3.2 Implementar `renderBalanceMessage(DistributionBalance, valuesBySlug)` — delta + reafirmação do alvo + eco dos valores, na unidade do usuário; sem vazar "renda".
- [ ] 3.3 Implementar `personalizePrompt(monthlyBudgetCents)` — âncora do orçamento + 5 categorias (`categoryLabels`) + regra do ZERO.
- [ ] 3.4 Atualizar `methodologyPrompt` para anunciar "não → personalizar", preservando o texto "Aceita esta sugestão".
- [ ] 3.5 Anexar aviso único de categorias zeradas ao `summaryPrompt` quando houver BP 0 (onboarding-only).

## Detalhes de Implementação

Ver `techspec.md` seções "Modelos de Dados" (schema de intenção de dois passos) e "Design de Implementação"; ADR-002 (classificação onboarding-only, precedência, aviso onboarding-only) e ADR-001 (render do saldo). Structured Output estrito (`llm.Schema{Strict:true}`); LLM só na call-site sancionada. Zero comentários (R-ADAPTER-001.1).

## Critérios de Sucesso

- Classificação retorna intenção fechada + `mixed_unit`; precedência `values` > `personalize` > `accept` (recusa+valores → `values`) — RF-01/RF-10.
- `personalizePrompt` cita o orçamento e as 5 categorias com rótulos e a regra do ZERO (RF-03); `methodologyPrompt` anuncia personalizar e mantém "Aceita esta sugestão" (RF-02, preserva `onboarding_workflow_test.go:1386`).
- Aviso de zero aparece apenas quando há categoria em 0 e apenas no onboarding (RF-07); `budget_creation` intocado.
- NR-06: nenhuma mensagem renderizada vaza "renda".

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — schema/prompt e classificação no consumidor `internal/agents`; LLM só na call-site sancionada, sem tocar o schema compartilhado.
- `domain-modeling-production` — intenção como tipo fechado; render separado da decisão (state-as-type).

## Testes da Tarefa

- [ ] Testes unitários: mapeamento de intenção (accept/personalize/values, precedência, mixed_unit), `renderBalanceMessage` (unidade correta, eco, sem "renda"), `personalizePrompt`/`methodologyPrompt`/aviso de zero. Package whitebox testify/suite com `agent.Agent` mockado.
- [ ] Testes de integração: não aplicável nesta tarefa.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/onboarding_workflow.go` (schema/prompt de intenção, render, copy, aviso)
- `internal/agents/application/workflows/onboarding_workflow_test.go` (testes)
