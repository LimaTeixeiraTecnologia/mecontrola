# Tarefa 4.0: Tools de operação diária (registrar/consultar/editar/remover/ajustar/classificar)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar as tools finas da operação diária em `internal/agents/application/tools`, cada uma com responsabilidade única, delegando aos bindings (tarefa 2.0) e à idempotência (tarefa 3.0). Escrita **só via `transactions`**; orçamento atualiza automaticamente via consumers (D-13). Data não informada → **hoje em America/Sao_Paulo** (ADR-008).

<requirements>
- ADR-003 (tool fina), ADR-004 (idempotência), ADR-008 (timezone).
- Tools: `register_expense`, `register_income`, `register_card_purchase`, `query_month`, `query_plan`, `edit_entry`, `delete_entry`, `adjust_allocation`, `classify_category`.
- Cobre: RF-20, RF-21, RF-21.1, RF-21.2, RF-22, RF-23, RF-24, RF-25, RF-25.1, RF-25.2, RF-26, RF-32, RF-35, RF-36.
</requirements>

## Subtarefas

- [ ] 4.1 `register_expense`/`register_income` (`tool.NewTool[I,O]`): inferir categoria via `classify_category` (dicionário); data-default hoje (SP); meio de pagamento obrigatório (RF-22); idempotente por `(wamid,item_seq,op)`.
- [ ] 4.2 `register_card_purchase`: parcelamento 1–24 (RF-24); resolve cartão por apelido; idempotente.
- [ ] 4.3 Suporte a **múltiplos lançamentos por mensagem** (RF-21.2): `item_seq` por item; resumo consolidado (✅); cada item idempotente.
- [ ] 4.4 `query_month`/`query_plan`: resumo mensal e planejado×gasto por categoria + alertas; responder do estado consolidado (nota de consistência eventual).
- [ ] 4.5 `edit_entry`/`delete_entry`: resolução do alvo no mês corrente por padrão, ampliando se citado (RF-26); marca a operação como destrutiva → delega ao gate HITL (tarefa 5.0).
- [ ] 4.6 `adjust_allocation`: reajuste de distribuição por conversa (`EditCategoryPercentage`, rebalanceia) (RF-25.1).
- [ ] 4.7 `classify_category`: `SearchDictionary` (exact/token/fuzzy); confirma quando ambíguo (RF-21/RF-32).

## Detalhes de Implementação

Ver techspec.md → "Interfaces Chave", ADR-003/004/008. `tool.NewTool[I,O]` com schemas `Strict:true`; `exec` determinístico delega a binding; sem regra/SQL/branching (R-AGENT-WF-001.2).

## Critérios de Sucesso

- Tools finas: zero regra de negócio/SQL/branching de domínio; wrapping `%w`; zero comentários (R-ADAPTER-001).
- `ToolOutcome` fechado (DMMF state-as-type); data-default em America/Sao_Paulo inline (sem abstrair tempo).
- Escrita exclusivamente via `transactions`; **sem chamar `budgets.UpsertExpense`** (D-13).
- Teste de integração prova: lançamento → `budgets_expenses` → `GetMonthlySummary` reflete **sem dupla contagem**.
- Múltiplos lançamentos: cada item idempotente por `item_seq`; resumo reporta sucessos/falhas.
- Build/gofmt/governança verdes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — criação de tools (`tool.NewTool[I,O]`) e roteamento Tool→binding→usecase no substrato do agente.

## Testes da Tarefa

- [ ] Testes unitários (testify/suite whitebox, IIFE por mock dos bindings): sucesso, erro de binding, validação de input, replay idempotente, data ausente→hoje (SP), múltiplos itens.
- [ ] Testes de integração (testcontainers): propagação lançamento→orçamento sem dupla contagem.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/tools/` (novas tools)
- Depende de `internal/agents/application/interfaces` (2.0) e `internal/agents/infrastructure/persistence` (3.0)
- techspec.md (Interfaces Chave), ADR-003/004/008
