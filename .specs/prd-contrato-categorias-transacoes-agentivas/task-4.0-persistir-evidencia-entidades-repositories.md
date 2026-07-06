# Tarefa 4.0: Persistir Evidencia em Entidades e Repositories

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Atualizar entidades, workflows de dominio e repositories Postgres de `internal/transactions` para carregar, persistir e reconstituir `CategoryWriteEvidence` em transacoes e templates recorrentes.

<requirements>
RF-17, RF-19, RF-21, RF-22, RF-23, RF-28, RF-29, RF-30.
RNF-01, RNF-03.
CA-16, CA-18, CA-19, CA-21, CA-22, CA-23.
</requirements>

## Subtarefas

- [ ] 4.1 Atualizar `Transaction` e `RecurringTemplate` para armazenar evidencia funcional junto dos snapshots.
- [ ] 4.2 Remover possibilidade de novo write categorizado com `subcategory_id` ausente.
- [ ] 4.3 Atualizar constructors/reconstitute/workflows de dominio para receber evidencia valida.
- [ ] 4.4 Atualizar repositories de transacoes e templates para insert, update, get, list e search com todas as colunas de evidencia.
- [ ] 4.5 Garantir que update sem troca de categoria reconstitui e substitui evidencia quando o gate retornar nova decisao.
- [ ] 4.6 Ajustar testes existentes que dependem de snapshots antigos.

## Detalhes de Implementação

Seguir `techspec.md`, secoes "Persistencia", "Contrato de transactions" e "Abordagem de Testes". Aplicar `go-implementation` para repository e domain model. DMMF e obrigatorio para preservar invariantes nos objetos de dominio e evitar campos opcionais que representem estado impossivel.

## Critérios de Sucesso

- Transacao e template recorrente nao perdem evidencia ao persistir ou reconstituir.
- `category_decided_at`, editorial version, source, score, confidence, quality, signal, matched term e reason persistem corretamente.
- Repository nao decide regra de negocio; apenas persiste estado ja validado.
- Repositories falham de forma rastreavel quando constraints do banco rejeitam estado invalido.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] `go test -race -count=1 ./internal/transactions/domain/...`
- [ ] `go test -race -count=1 ./internal/transactions/infrastructure/repositories/postgres/...`
- [ ] Integration tests de create/get/list/search/update para transacoes e templates com evidencia completa.
- [ ] Teste garantindo que update altera evidencia e `category_decided_at` quando o gate retorna nova decisao.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/transactions/domain/entities/transaction.go`
- `internal/transactions/domain/entities/recurring_template.go`
- `internal/transactions/domain/services/transaction_workflow.go`
- `internal/transactions/infrastructure/repositories/postgres/transaction_repository.go`
- `internal/transactions/infrastructure/repositories/postgres/recurring_template_repository.go`
- `internal/transactions/infrastructure/repositories/postgres/transaction_repository_integration_test.go`
- `internal/transactions/infrastructure/repositories/postgres/recurring_template_repository_integration_test.go`
