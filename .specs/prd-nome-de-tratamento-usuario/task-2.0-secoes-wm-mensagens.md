# Tarefa 2.0: Helpers de seção de working memory e mensagens determinísticas

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Adicionar aliases finos e genéricos de manipulação de seções `##` do working_memory (reutilizando os helpers já existentes do goal-edit, sem alterá-los) e os builders de mensagem determinísticos do fluxo (boas-vindas+pergunta do nome no onboarding, prompt de nova pergunta na edição, confirmação verbatim), todos no Tom de Voz oficial.

<requirements>
RF-03 (persistência em seção de conteúdo), RF-09 (substituição de seção preservando irmãs), RF-12 (mensagens verbatim no Tom de Voz oficial).
</requirements>

## Subtarefas

- [ ] 2.1 Criar `internal/agents/application/workflows/working_memory_sections.go` com `parseWorkingMemorySections`/`replaceWorkingMemorySection`/`workingMemorySectionBody` que encapsulam `goalEditParseSections`/`goalEditReplaceSection`/`goalEditSectionBody` (mesmo package `workflows`), sem modificar o goal-edit.
- [ ] 2.2 Adicionar em `internal/agents/application/messages/catalog.go` builders determinísticos: confirmação de troca "Combinado, %s! 💚 Vou te chamar assim daqui pra frente."; prompt de nova pergunta "Claro! Como você gostaria que eu te chamasse a partir de agora? 💚".
- [ ] 2.3 Adicionar em `internal/agents/application/workflows/onboarding_workflow.go` (consts, apenas texto) a mensagem de boas-vindas + pergunta do nome ("Antes da gente começar, como você gostaria que eu te chamasse? 💚") e o prompt de objetivo sem re-saudação (a lógica que os usa é da Tarefa 4.0).
- [ ] 2.4 Testes dos builders/aliases.

## Detalhes de Implementação

Ver `techspec.md` seções "Componentes" e "Design de Implementação"; helpers de seção em `internal/agents/application/workflows/goal_edit_workflow.go:197-267`; catálogo `internal/agents/application/messages/catalog.go` (padrão `pick`/`NewMotivationSeed`). ADR-001, ADR-003.

## Critérios de Sucesso

aliases não duplicam lógica (encapsulam os existentes); mensagens verbatim conforme US/PRD; asterisco simples e emoji oficial (Tom de Voz); zero comentários; testes verdes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `design-patterns-mandatory` — Facade/aliases finos sobre os helpers de seção existentes, sem duplicar lógica.
- `mastra` — mensagens determinísticas verbatim do agente/consumidor no Tom de Voz oficial.

## Testes da Tarefa

- [ ] Testes unitários (aliases de seção preservando irmãs; builders de mensagem)
- [ ] Testes de integração: não aplicável

<critical>SEMPRE CRIAR E EXECUTAR TESTES antes de marcar a tarefa como concluída</critical>

## Arquivos Relevantes

- `internal/agents/application/workflows/working_memory_sections.go` (novo)
- `internal/agents/application/messages/catalog.go` (mod)
- `internal/agents/application/workflows/onboarding_workflow.go` (consts)
- testes correspondentes
- referência `internal/agents/application/workflows/goal_edit_workflow.go:197-267`
