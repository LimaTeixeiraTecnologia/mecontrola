# Tarefa 7.0: Heurística de múltiplos lançamentos + slot de forma de pagamento

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Corrigir o falso positivo de "múltiplos lançamentos" com número BRL formatado (R5) e endurecer o
slot de forma de pagamento: perguntar com exemplos, proibir inferência e corrigir o mapa de códigos
(R8). Ver techspec.md (R5/R8).

<requirements>
- RF-19: a detecção de múltiplos lançamentos ignora separadores de milhar/decimal internos a um único valor.
- RF-20: mensagem com um único valor e um único item não dispara múltiplos lançamentos.
- RF-21: dois ou mais valores/itens com conectores continuam disparando, com o texto de orientação inalterado.
- RF-29: despesa sem forma de pagamento não assume "dinheiro" nem padrão.
- RF-30: o agente pergunta a forma de pagamento com exemplos; prompt do slot e reprompt incluem exemplos.
- RF-31: as instruções proíbem inferir/inventar a forma de pagamento não declarada.
- RF-32: mapeamento texto→código segue o enum fechado; receita não pede forma de pagamento.
</requirements>

## Subtarefas

- [ ] 7.1 Ajustar a heurística no prompt (`mecontrola_agent.go:15-16`): ponto é separador de milhar
  no padrão brasileiro (R$ 1.234,56 é UM valor); conectores ("e"/"mais"/"também"/vírgula entre itens)
  e o texto de orientação permanecem inalterados.
- [ ] 7.2 Prompt inicial do slot de pagamento (`pending_entry_workflow.go:680`) passa a incluir
  exemplos; instruções do agente proíbem inferir forma de pagamento ausente.
- [ ] 7.3 Corrigir `knownPaymentMethods` (`pending_entry_decisions.go:118`): `"doc":"doc"` produz
  código fora do enum de `register_expense` — mapear para `ted` (ou remover a entrada).
- [ ] 7.4 Confirmar que income não pede forma de pagamento (`confirmPaymentSegment` vazio;
  `registerIncomePaymentMethod = "pix"`).

## Detalhes de Implementação

Ver techspec.md "Considerações Técnicas → R5/R8". `PaymentMethod` já é tipo fechado com smart
constructor `ParsePaymentMethodForCreate` (defesa final, não-silenciosa após 3.0). Nenhum novo tipo.

## Critérios de Sucesso

- "R$ 13.874,40" não dispara múltiplos; "gastei 30 no ônibus e 15 no café" dispara com texto exato.
- Despesa sem forma de pagamento pergunta com exemplos; nunca assume "dinheiro".
- Receita não pergunta forma de pagamento; `knownPaymentMethods` só produz códigos válidos.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — instruções do agente, tools de escrita e slots do pending workflow no consumidor `internal/agents`.
- `domain-modeling-production` — `PaymentMethod` como tipo fechado e mapeamento texto→código sem valor ilegal.
- `design-patterns-mandatory` — gate `não aplicar padrão` (ajuste de prompt e correção de mapa).

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/agents/mecontrola_agent.go`
- `internal/agents/application/workflows/pending_entry_workflow.go`, `pending_entry_decisions.go`
- `internal/agents/application/tools/register_expense.go`, `register_income.go`
- `internal/transactions/domain/valueobjects/payment_method.go`
