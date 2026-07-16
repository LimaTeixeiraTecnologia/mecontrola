# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Edição do nome de tratamento como workflow durável dedicado `treatment-name-edit`, sem gate de confirmação
- **Data:** 2026-07-15
- **Status:** Aceita
- **Decisores:** Autor do PRD/techspec, confirmado pelo solicitante (múltipla escolha)
- **Relacionados:** `prd.md` (RF-06, RF-07, RF-08, RF-09, RF-13), `techspec.md`, `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001.1/.2/.3/.4/.7), ADR-001, ADR-003

## Contexto

- RF-08 exige interação de dois turnos (usuário pede troca sem informar o nome → agente pergunta uma vez → aplica), com retomada determinística. RF-07 exige turno único quando o nome já vem na mensagem. Não há gate sim/não (decisão de produto): personalização não-destrutiva, fora do escopo HITL do `R-AGENT-WF-001.7-A`.
- Blueprint existente: `goal-edit` é um workflow durável com tool de start (`edit_goal.go:27-89`), resume via `ResumeDispatcher` (`resume_dispatcher.go`) → `WorkflowResumer` → `ContinueGoalEdit` (`goal_edit_workflow.go:269-295`), estados fechados (`goal_edit_state.go`), `Decide*` puro (`goal_edit_decisions.go`), TTL/reaper e idempotência por `MessageID`.
- A ordem de resolução do inbound é resume-antes-do-parse: `whatsapp_inbound_consumer.go:149-153` roda `tryDispatchResume` → `tryResolveOnboarding` → `handleAgentInbound` (LLM). O `SuspendedRunIndex` só resolve runs `Suspended` e impõe um-fluxo-por-recurso (`suspended_run_index.go:36-38`).

## Decisão

Implementar um workflow durável dedicado `treatment-name-edit`, espelhando o `goal-edit`, **sem o slot de confirmação sim/não**. Diferenças em relação ao `goal-edit`:

- A tool `edit_treatment_name` recebe input tipado `{name string}` (opcional), ao contrário do `edit_goal` (input vazio). Isso satisfaz RF-07 em turno único: com `name` usável, o step aplica e conclui imediatamente; sem `name`, suspende perguntando "Claro! Como você gostaria que eu te chamasse a partir de agora? 💚".
- Um único slot de captura de nome (sem `Awaiting`), pois não há segundo estado de confirmação. Ciclo de vida em tipo fechado `TreatmentNameEditStatus`.
- Extração do nome no resume via `agent.Execute` com schema (call-site sancionada de workflow step, como o onboarding), porque respostas como "me chama de Stef" exigem extração — diferentemente do goal-edit que trata o texto inteiro como objetivo.
- Confirmação verbatim "Combinado, `<nome>`! 💚 Vou te chamar assim daqui pra frente." (RF-07/RF-08).
- Mantidos do blueprint: reprompt-em-vazio (1×), TTL 15 min de decisão + reaper 20 min, resume-antes-do-parse via `ContinueTreatmentNameEdit`, wiring engine/registry/resumer/reaper, e a inclusão no `SuspendedRunIndex`/`ResumeDispatcher`.

Idempotência: como o `SuspendedRunIndex` só resolve runs `Suspended`, um replay do mesmo `wamid` após a conclusão não reabre o run (cai no parse do agente); o `Upsert`/`UpsertMetadata` do nome é idempotente por natureza (overwrite). `MessageID` é persistido para auditoria.

## Alternativas Consideradas

- **Tool única `edit_treatment_name` (sem workflow).** Vantagem: mínima para RF-07. Desvantagem: RF-08 (dois turnos) perde estado durável e a garantia resume-antes-do-parse; o "perguntar uma vez e aplicar" passaria a depender do LLM lembrar entre turnos. Rejeitada por fragilidade e não-determinismo.
- **Híbrido tool (RF-07) + workflow (RF-08).** Desvantagem: duplica extração/persistência em dois caminhos, com risco de divergência. Rejeitada.
- **Reutilizar o próprio `goal-edit` genérico.** Desvantagem: acoplaria semânticas distintas e exigiria um discriminador de tipo de operação. Rejeitada — clareza e fronteiras.

## Consequências

### Benefícios Esperados

- Determinismo e robustez para os dois turnos do RF-08; reuso máximo de um padrão já validado em produção.
- Aderência de governança (registry, estados fechados, LLM só sancionado, Run auditável, resume-antes-do-parse).

### Trade-offs e Custos

- Um novo engine/registry/resumer/reaper por tipo de estado (custo de wiring, já padronizado no `module.go`).
- Extração via LLM no resume adiciona uma chamada por turno de edição (aceitável; provider único OpenRouter).

### Riscos e Mitigações

- Risco: colisão com outro fluxo suspenso (um-fluxo-por-recurso). Mitigação: comportamento desejado e já modelado (`ErrMultipleSuspendedRuns`).
- Risco: expiração deixando run pendurado. Mitigação: reaper `agents-treatment-name-edit-reaper` + expiry retorna `handled=false` para o texto seguir ao parse.
- Rollback: remover tool do agente e do write-set + desregistrar workflow; runs suspensos são drenados pelo reaper.

## Plano de Implementação

1. Estado + tipo fechado + `Decide*` puros (com testes de suíte).
2. Workflow (step, execute, Continue) + testes.
3. Tool fina + testes.
4. Wiring no `module.go` (engine/registry/resumer/reaper/tool/write-set/index/dispatcher) + `regression_contract.go`.
5. Casos golden + gate real-LLM.

## Monitoramento e Validação

- `agents_resume_dispatch_total{workflow="treatment-name-edit", outcome}` + histograma de duração; Run auditável por execução.
- Critério: gate golden ≥ 0,90 na categoria; unit cobrindo turno único, dois turnos, reprompt, expiry.
- Reverter a decisão se a taxa de `outcome=failed` do dispatcher exceder o baseline do `goal-edit`.

## Impacto em Documentação e Operação

- Runbook de agents: novo workflow, reaper e tool; contrato de regressão atualizado.

## Revisão Futura

- Revisitar se produto decidir introduzir confirmação (reintroduziria escopo HITL do `R-AGENT-WF-001.7-A`).
