# Tarefa 1.0: Domínio transactions — enriquecer `TransactionUpdated` + `DecideUpdate` popular campos + no-op

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Enriquecer o evento de domínio `TransactionUpdated` (aditivo, compatível) e fazer `DecideUpdate` (puro) popular os campos novos a partir do comando e dos itens atuais, incluindo detecção de no-op. Base para o reflexo no orçamento (ADR-001).

<requirements>
- RF-27: `TransactionUpdated` passa a carregar `CategoryID`, `SubcategoryID`, `Installments` (conjunto atual) e `PreviousItemIDs` (itens antigos).
- RF-24: editar compra parcelada recompõe todas as parcelas (compra inteira) — já em `DecideUpdate`; garantir e testar.
- RF-22: no-op (valores confirmados idênticos aos atuais) detectado em `DecideUpdate`, sem gravar/incrementar version.
- RF-15: migração para fora de `credit_card` bloqueada quando há parcelas em aberto — garantir invariante e teste.
- `DecideUpdate` permanece puro (sem IO/ctx, determinístico); regra de negócio só aqui (R-TXN-001).
- `RefMonthsAffected` = competências antigas ∪ novas (RF-15/RF-27).
</requirements>

## Subtarefas

- [ ] 1.1 Adicionar `CategoryID`, `SubcategoryID`, `Installments []CardPurchaseInstallment`, `PreviousItemIDs []uuid.UUID` a `TransactionUpdated` (`internal/transactions/domain/entities/events.go:26-36`), com json tags.
- [ ] 1.2 Popular os campos novos nos dois ramos de `DecideUpdate` (não-cartão e cartão) a partir de `cmd` e `currentItems` (`transaction_workflow.go:164-176,240-250`), mantendo pureza.
- [ ] 1.3 Detecção de no-op: quando os campos resultantes forem idênticos ao atual, sinalizar decisão sem mutação/evento de escrita (contrato consumido por 8.0).
- [ ] 1.4 Testes unitários puros (zero mock) cobrindo não-cartão, cartão, migração pix↔crédito, no-op, `RefMonthsAffected` e `PreviousItemIDs`.

## Detalhes de Implementação

Ver `techspec.md` (Interfaces Chave — evento enriquecido; Modelos de Dados) e `adr-001-budget-reconcile-updated-event.md`. Nenhuma migração de DB (evento é JSON no outbox).

## Critérios de Sucesso

- `DecideUpdate` popula os quatro campos novos corretamente em todos os cenários; permanece puro e determinístico.
- No-op não produz mutação nem evento de escrita.
- Gate R-TXN-001 (regra fora de `Decide*`) e R-TXN-003 permanecem verdes.
- `go build`, `go vet`, `go test -race` do módulo verdes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `domain-modeling-production` — evento de domínio + `Decide*` puro com state-as-type e detecção de no-op.
- `design-patterns-mandatory` — gate de desenho (selector = reject; nenhum GoF novo, reuso inline).

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/transactions/domain/entities/events.go`
- `internal/transactions/domain/services/transaction_workflow.go`
- `internal/transactions/domain/services/transaction_workflow_test.go`
