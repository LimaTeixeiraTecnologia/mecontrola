# Tarefa 4.0: Reforçar `pending-entry` para pix sem cartão e receita simples

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Garantir que o fluxo `pending-entry` confirme e persista despesas pix sem depender de cartão, e que receitas simples com valor BRL usando separador de milhar sejam tratadas como lançamento único com descrição literal preservada.

<requirements>
- RF-17: despesa pix com valor, descrição, categoria e data resolvida pede confirmação antes de persistir.
- RF-18: após confirmação positiva de despesa pix, persistir transação ativa com direção despesa, valor em centavos, descrição literal, payment_method pix, categoria decidida, `origin_wamid` e `origin_operation`.
- RF-19: ausência de cartão ativo não impede confirmação nem persistência de despesa pix.
- RF-20: receita simples como "Recebi R$ 13.874,40 de salário" não pode virar múltiplos lançamentos.
- RF-21: termo literal da receita (ex.: "salário") deve ser preservado como descrição sem parafrasear.
- RF-22: receita simples deve registrar uma única intenção de receita ou iniciar apenas a confirmação mínima necessária.
- RF-23: após confirmação positiva, existe transação ativa de receita com valor em centavos correto e descrição literal.
- RF-26: toda escrita financeira coberta passa por confirmação humana antes da persistência.
- RF-27: reenvio ou retomada da mesma pendência não cria escrita duplicada; origem rastreável por `origin_wamid` e operação.
</requirements>

## Subtarefas

- [ ] 4.1 Validar que `DecideInitialAwaiting` só retorna `AwaitingSlotCard` quando `paymentMethod == "credit_card"` e `hasCard == false`.
- [ ] 4.2 Confirmar que despesa pix com categoria resolvida chega a confirmação sem cartão.
- [ ] 4.3 Garantir que `buildRawTransaction` preserve `PaymentMethod`, `OriginWamid` e `OriginOperation` após confirmação positiva.
- [ ] 4.4 Validar que `mecontrola_agent.go` mantém regra anti falso múltiplo lançamento para BRL com separador de milhar.
- [ ] 4.5 Verificar idempotência de retomada de pendência por `origin_wamid` e operação.

## Detalhes de Implementação

Ver `techspec.md` — seções **Visão Geral dos Componentes** (pending-entry) e **Testes Unitários / Pending-entry**. A regra central já deve estar correta; esta tarefa é reforço de regressão com testes e possíveis ajustes pontuais nas decisões.

## Critérios de Sucesso

- Teste unitário confirma que `DecideInitialAwaiting` só exige cartão em `credit_card` sem cartão.
- Teste de integração confirma que despesa pix confirmada cria transação ativa com `amount_cents=5000`, descrição literal, `payment_method=pix`, categoria compatível e `origin_wamid` preenchido.
- Teste unitário/golden confirma que "Recebi R$ 13.874,40 de salário" não ativa `multi_item`.
- Teste de integração confirma que receita confirmada cria transação ativa com `amount_cents=1387440` e descrição "salário".
- `go test -race -count=1 ./internal/agents/application/workflows/... ./internal/agents/application/agents/...` passa.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — alteração no workflow durável `pending-entry` e agente financeiro.
- `domain-modeling-production` — decisões puras de domínio (`DecideInitialAwaiting`, `buildRawTransaction`, idempotência).

## Testes da Tarefa

- [ ] Testes unitários das decisões de pending-entry.
- [ ] Testes de integração de pix confirmado e receita simples.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/pending_entry_decisions.go`
- `internal/agents/application/workflows/pending_entry_workflow.go`
- `internal/agents/application/workflows/pending_entry_workflow_test.go`
- `internal/agents/application/agents/mecontrola_agent.go`
- `internal/transactions/application/usecases/create_transaction.go`
- `internal/transactions/infrastructure/repositories/postgres/transaction_repository.go`
