# Tarefa 5.0: Implementar CategoryWriteGate nos Use Cases

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar `CategoryWriteGate` como interface consumidora de `transactions`, implementar adapter fino para `categories` e aplicar o gate em `CreateTransaction`, `UpdateTransaction`, `CreateRecurringTemplate` e `UpdateRecurringTemplate`.

<requirements>
RF-01, RF-03, RF-04, RF-06, RF-09, RF-10, RF-11, RF-15, RF-16, RF-17, RF-20, RF-23, RF-28, RF-29, RF-32, RF-35.
RNF-01, RNF-02, RNF-04, RNF-05.
CA-06, CA-07, CA-08, CA-11, CA-13, CA-15, CA-17, CA-23.
</requirements>

## Subtarefas

- [ ] 5.1 Criar `CategoryWriteGate` e `CategoryWriteGateInput` em `transactions/application/interfaces`.
- [ ] 5.2 Implementar adapter de infraestrutura que chama `categories.ResolveCategoryForWrite`.
- [ ] 5.3 Mapear direction para kind esperado sem string livre critica.
- [ ] 5.4 Bloquear root sem leaf, deprecated, kind/direction mismatch, version drift e evidencia ausente antes do repository.
- [ ] 5.5 Aplicar gate nos quatro use cases de escrita.
- [ ] 5.6 Revalidar e atualizar evidencia em todo update, mesmo quando categoria/subcategoria nao mudarem.
- [ ] 5.7 Atualizar wiring em `internal/transactions/module.go`.
- [ ] 5.8 Regenerar mocks quando interface consumidora mudar.

## Detalhes de Implementação

Seguir `techspec.md`, secoes "Contrato de transactions", "Regras de Gate" e "Pontos de Integracao". Interface deve ficar no consumidor (`transactions`). Adapter deve ser fino e sem regra de negocio fora do use case/gate. Aplicar DMMF: decisoes funcionais fechadas, command object para write use cases e erros tipados.

## Critérios de Sucesso

- Todo write categorizado passa por `CategoryWriteGate.Approve` imediatamente antes de persistir.
- Manual nao agentivo usa `manual_canonical_id` e evidencia deterministica.
- Updates sem mudanca de categoria ainda chamam o gate.
- Templates recorrentes tem paridade com transacoes diretas.
- `CategoriesCache`, se permanecer, nao autoriza escrita sozinho.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] `go test -race -count=1 ./internal/transactions/...`
- [ ] Unit tests dos quatro use cases bloqueando sem evidencia, root sem leaf, deprecated, kind incompativel, version drift e subcategoria fora da raiz.
- [ ] Unit test de update sem troca de categoria provando chamada ao gate e persistencia de evidencia nova.
- [ ] Integration com `categories` real para evitar mock aceitando kind invalido.
- [ ] `go vet ./internal/transactions/...`

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/transactions/application/interfaces/category_write_gate.go`
- `internal/transactions/application/interfaces/types.go`
- `internal/transactions/application/usecases/create_transaction.go`
- `internal/transactions/application/usecases/update_transaction.go`
- `internal/transactions/application/usecases/create_recurring_template.go`
- `internal/transactions/application/usecases/update_recurring_template.go`
- `internal/transactions/infrastructure/repositories/postgres/categories_reader_adapter.go`
- `internal/transactions/infrastructure/config/categories_cache.go`
- `internal/transactions/module.go`
