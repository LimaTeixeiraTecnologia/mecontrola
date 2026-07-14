# Tarefa 2.0: Decisão pura de saldo e refactor da conversão em basis points

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar o núcleo compartilhado da feature: a decisão pura de saldo (passou/faltou/dentro-da-tolerância) como tipo fechado, e refatorar a conversão em basis points para fechar 10000 por maior-resto (percentual e reais), absorvendo arredondamento. Move as mensagens de over/under para fora de `DecideAllocationsBP`.

<requirements>
- RF-04: soma acima do total informa quanto passou, reafirma o alvo, ecoa valores e pede redistribuição, sem ativar.
- RF-05: soma abaixo do total informa quanto falta, reafirma o alvo, ecoa valores e orienta, sem ativar.
- RF-06: delta na mesma unidade usada pelo usuário (% ou R$).
- RF-09: tolerância ±0,5% / ±R$0,05 sobre a soma bruta, com absorção do resto na maior categoria.
- RF-11: invariante de fechamento preservado (basis points somam 10000).
</requirements>

## Subtarefas

- [ ] 2.1 Implementar a struct `DistributionBalance` e a função pura `DecideDistributionBalance(kind, valuesBySlug, monthlyBudgetCents) DistributionBalance` (sem IO, sem `context.Context`, determinística), com tolerância ±0,5 (percentual) / ±R$0,05 (reais) sobre a soma bruta.
- [ ] 2.2 Refatorar `DecideAllocationsBP` para fechar 10000 por maior-resto em percentual e reais (reaproveitando `centsToBasisPoints`); remover as mensagens de over/under (agora no saldo); manter rejeição de negativo, confirm-com-valores e orçamento ausente.
- [ ] 2.3 Definir as constantes de tolerância (percentual e reais) sem prefixo `_` e sem comentário.
- [ ] 2.4 Manter `DecideBudgetDistribution` (sum=10000) como rede de segurança pós-conversão.

## Detalhes de Implementação

Ver `techspec.md` seções "Interfaces Chave" e "Design de Implementação"; ADR-001 (híbrido DMMF, gate de pattern `reject`) e ADR-003 (tolerância). `Decide*` puro (DMMF): sem IO, sem log, determinístico, testável sem mock. Aplicar `errors.Join`/`%w` (R7.6/R5.10). Zero comentários (R-ADAPTER-001.1).

## Critérios de Sucesso

- `DecideDistributionBalance` retorna `over`/`under`/`balanced` corretos, com `DeltaAbs` e `Unit` na unidade do usuário (RF-04/05/06).
- NR-01: para somas exatas (100% ou orçamento), os basis points produzidos são idênticos aos de hoje (`40/10/10/10/30 → {4000,1000,1000,1000,3000}`; reais exatos idênticos ao caminho `centsToBasisPoints`) — teste comparando BP antes/depois.
- RF-09: `33,3+33,3+33,4` e reais a até R$0,05 do orçamento fecham 10000 por absorção; fora da banda cai em over/under.
- RF-11: `DecideDistribution` (sum=10000) nunca é violado após a conversão.
- NR-06: nenhuma mensagem produzida vaza a palavra "renda".

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `domain-modeling-production` — modelar a decisão de saldo como função pura + tipo fechado (Decide* puro, state-as-type).
- `design-patterns-mandatory` — materializar o gate de pattern desta decisão (resultado reject, ou seja, não aplicar padrão), registrando alternativas GoF rejeitadas.
- `mastra` — o núcleo é consumido por dois workflows do substrato; sem LLM nas funções puras.

## Testes da Tarefa

- [ ] Testes unitários: `DecideDistributionBalance` (over/under/balanced, unidade, delta), `DecideAllocationsBP` (maior-resto percent e reais, tolerância, negativo, confirm-com-valores, orçamento ausente), e comparação BP antes/depois para somas exatas (NR-01). Package whitebox testify/suite.
- [ ] Testes de integração: não aplicável nesta tarefa.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/onboarding_workflow.go` (`DecideDistributionBalance`, `DecideAllocationsBP`, `centsToBasisPoints`)
- `internal/agents/application/workflows/budget_creation_decisions.go` (`DecideBudgetDistribution` rede de segurança)
- `internal/agents/application/workflows/onboarding_workflow_test.go`, `budget_creation_decisions_test.go` (testes)
