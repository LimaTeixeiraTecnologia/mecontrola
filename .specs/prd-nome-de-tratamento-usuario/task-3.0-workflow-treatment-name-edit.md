# Tarefa 3.0: Workflow durável treatment-name-edit sem gate de confirmação

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar o workflow durável dedicado de alteração do nome de tratamento pós-onboarding, espelhando o goal-edit SEM o slot de confirmação sim/não: com nome informado aplica em turno único; sem nome, suspende perguntando e retoma por merge-patch antes do parse. Persiste na seção de working memory (writer via merge de seção) + metadata, e confirma verbatim.

<requirements>
- RF-06: reconhece intenção de troca por linguagem natural.
- RF-07: nome na mensagem → turno único.
- RF-08: sem nome → pergunta uma vez → aplica.
- RF-09: aplicação imediata.
- RF-10: isolamento — só toca platform_resources, nunca identity.
- RF-13: falha de persistência → StepStatusFailed sem confirmar sucesso.
</requirements>

## Subtarefas

- [ ] 3.1 Criar `internal/agents/application/workflows/treatment_name_edit_workflow.go`: `TreatmentNameEditWorkflowID="treatment-name-edit"`, `TreatmentNameEditKey(resourceID, threadID)` via `CorrelationKey`, `BuildTreatmentNameEditWorkflow(workingMem memory.WorkingMemory, a agent.Agent)` (Definition Durable, MaxAttempts 1), constantes `treatmentNameEditTTL`(15min)/StaleAfter(20min)/ReaperBatch(100)/maxReprompts(1).
- [ ] 3.2 Step: se `ProvidedName` usável (`DecideTreatmentName`)→`executeTreatmentNameEdit` (turno único); senão se `ResumeText==""`→suspende com o prompt de pergunta; senão extrai nome via `a.Execute` com schema estrito (call-site sancionada) + `DecideTreatmentName`→execute; vazio→reprompt(1×) depois cancela; expiry via `DecideTreatmentNameEditExpiry`→handled=false.
- [ ] 3.3 `executeTreatmentNameEdit`: `Get` conteúdo, `replaceWorkingMemorySection(content, "## Nome de Tratamento", newName)`, `Upsert`, `UpsertMetadata({"nome_tratamento": newName})`; falha em qualquer→`StepStatusFailed` + ResponseText não-sucesso; sucesso→ResponseText verbatim "Combinado, %s! 💚 Vou te chamar assim daqui pra frente." (RF-13/RF-07).
- [ ] 3.4 `ContinueTreatmentNameEdit(ctx, engine, def, key, userMessage) (bool,string,error)` espelhando `ContinueGoalEdit` (resume merge-patch `{"resumeText":...}`; status 0 & sem erro→não handled; expirado→handled=false). `BuildTreatmentNameEditReaper`.
- [ ] 3.5 Testes de workflow (fake `WorkingMemory` + fake `agent.Agent`): turno único; sem nome→suspende; resume com nome→Upsert+UpsertMetadata+ResponseText; resume vazio→reprompt→cancela; expiry→handled=false; falha de Upsert→StepStatusFailed.

## Detalhes de Implementação

Ver `techspec.md` (Interfaces Chave, Modelos de Dados, Riscos) e ADR-002. Blueprint: `internal/agents/application/workflows/goal_edit_workflow.go` (step/execute/Continue/reaper) e `goal_edit_workflow.go:269-295` (Continue). LLM só na call-site sancionada de extração (como onboarding `onboarding_workflow.go:1027`); nenhum LLM em Decide/kernel. Merge de seção preserva `## Objetivo Financeiro`. Sem confirm slot (ADR-002). Idempotência estrutural: SuspendedRunIndex só resolve runs suspensos.

## Critérios de Sucesso

Workflow durável funcional; estados fechados; resume-antes-do-parse; RF-07 turno único e RF-08 dois turnos determinísticos; RF-13 falha explícita; zero comentários; adapter/step fino sem SQL/branching de domínio; testes verdes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — workflow durável no substrato de agent/kernel, resume-antes-do-parse e Run auditável.
- `domain-modeling-production` — Decide* puro e estados fechados do fluxo de edição.
- `design-patterns-mandatory` — gate de desenho do workflow e reuso de padrões do goal-edit.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

Unitários: workflow com fakes. Integração: coberta na Tarefa 7.0.

## Arquivos Relevantes

- `internal/agents/application/workflows/treatment_name_edit_workflow.go` (novo) + teste.
- Consome `treatment_name_edit_state.go`/`_decisions.go` (Tarefa 1.0), `working_memory_sections.go` + `messages/catalog.go` (Tarefa 2.0).
- Referência: `internal/agents/application/workflows/goal_edit_workflow.go`.
