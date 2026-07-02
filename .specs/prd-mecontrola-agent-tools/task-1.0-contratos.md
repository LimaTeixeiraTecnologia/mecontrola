# Tarefa 1.0: Contratos: interfaces de consumidor, tipos agent-owned e RecurrenceManager

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Estabelecer a camada de contratos na camada consumidora `internal/agents` antes de qualquer comportamento: estender as interfaces de consumidor existentes com os métodos novos, criar a interface coesa `RecurrenceManager` e definir os tipos agent-owned (structs planas espelhando os DTOs dos módulos). É a base compilável e isolada para as tools e adapters das tarefas seguintes. Ver techspec seção "Interfaces Chave", "Modelos de Dados" e ADR-004.

<requirements>
- RF-19: cada capacidade delega a um único use case de leitura/escrita real, sem branching de domínio nos contratos.
- Estender `card_manager.go`, `transactions_ledger.go`, `budget_planner.go` com os métodos da techspec seção "Interfaces Chave".
- RF-18e: estender `categories_reader.go` (`CategoriesReader`, hoje apenas `SearchDictionary`/`ResolveRootsBySlug`) com um método de **listagem de categorias** mapeado ao use case real `internal/categories/application/usecases/list_categories.go` (`ListCategories`), atendendo "quais categorias existem/disponíveis?". Definir o tipo agent-owned plano de saída (ex.: `Category`) espelhando o `ListCategoriesOutput` do módulo.
- Criar interface coesa `RecurrenceManager` (`recurrence_manager.go`, ADR-004) em vez de inflar `TransactionsLedger`.
- Definir tipos agent-owned planos: `Card`, `BestPurchaseDay`, `CardInvoice`, `CardUpdate`, `Recurrence`, `RawRecurrence`, `RawUpdateRecurrence`, `AllocationBP`, `AllocationCents`, `Category`.
- R-DTO-VALIDATE-001: qualquer novo input DTO em `application/dtos/input/` DEVE ter `Validate() error` com `errors.Join` e mensagem nomeando o campo.
- R-ADAPTER-001.1: zero comentários em Go de produção.
- Proibido asserção de interface em tempo de compilação `var _ I = (*T)(nil)` (memória `feedback_no_interface_assertion`).
- Atualizar `.mockery.yml` para as interfaces novas/alteradas e regenerar mocks.
</requirements>

## Subtarefas

- [ ] 1.1 Adicionar os métodos novos às 3 interfaces de consumidor (`card_manager.go`, `transactions_ledger.go`, `budget_planner.go`) conforme as assinaturas da techspec seção "Interfaces Chave".
- [ ] 1.2 Criar a interface coesa `RecurrenceManager` em `recurrence_manager.go` (ADR-004) com `CreateRecurrence`, `UpdateRecurrence`, `DeleteRecurrence`, `ListRecurrences`.
- [ ] 1.3 Estender `categories_reader.go` (`CategoriesReader`) com o método de listagem mapeado a `ListCategories` (RF-18e); definir o tipo agent-owned `Category` espelhando `ListCategoriesOutput`.
- [ ] 1.4 Definir os tipos agent-owned planos (`Card`, `BestPurchaseDay`, `CardInvoice`, `CardUpdate`, `Recurrence`, `RawRecurrence`, `RawUpdateRecurrence`, `AllocationBP`, `AllocationCents`) espelhando os DTOs dos módulos; adicionar `Validate()` a qualquer novo input DTO.
- [ ] 1.5 Atualizar `.mockery.yml` para as interfaces novas/alteradas e regenerar os mocks.

## Detalhes de Implementação

Ver techspec.md seção "Design de Implementação → Interfaces Chave" (assinaturas concretas dos métodos novos por interface e da nova `RecurrenceManager`), seção "Modelos de Dados" (lista dos tipos agent-owned e campos derivados dos DTOs verificados) e "Considerações Técnicas → ADR-004" (segregação de interface). As assinaturas devem bater literalmente com a techspec. Os tipos agent-owned são structs planas sem lógica, espelhando os DTOs dos módulos referenciados em "Arquivos Relevantes e Dependentes". Manter o idioma já existente das interfaces de `internal/agents/application/interfaces/`. `categories_reader.go` é estendida com o método de listagem (RF-18e) — os métodos existentes (`SearchDictionary`/`ResolveRootsBySlug`) permanecem inalterados.

## Critérios de Sucesso

- Pacote `internal/agents/application/interfaces/` compila isolado (sem comportamento).
- Assinaturas das interfaces e nomes/campos dos tipos agent-owned batem com a techspec.
- Zero comentários em código de produção (R-ADAPTER-001.1).
- Nenhuma asserção `var _ I = (*T)(nil)`.
- Novos input DTOs (se houver) possuem `Validate()` (R-DTO-VALIDATE-001).
- `.mockery.yml` atualizado e mocks gerados sem erro.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — port do agente sobre o substrato internal/platform; criar/estender tools, interfaces de consumidor e bindings segue o molde internal/agents.

## Testes da Tarefa

- [ ] Testes unitários — validado por compilação do pacote e geração de mocks (unit de mapeamento args↔DTO vem na Tarefa 2.0); `Validate()` de qualquer input DTO novo coberto por teste de tabela.
- [ ] Testes de integração — N/A nesta tarefa (camada de contrato sem IO).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/interfaces/card_manager.go`
- `internal/agents/application/interfaces/transactions_ledger.go`
- `internal/agents/application/interfaces/budget_planner.go`
- `internal/agents/application/interfaces/categories_reader.go` (estendida — RF-18e)
- `internal/agents/application/interfaces/recurrence_manager.go` (novo)
- `internal/agents/application/interfaces/types.go` (tipo agent-owned `Category`)
- `.mockery.yml`
