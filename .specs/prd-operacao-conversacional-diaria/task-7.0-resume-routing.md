# Tarefa 7.0: Roteamento de resume por registry (SuspendedRunIndex)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Substituir o `tryResumeChain` (slice hardcoded de 5 continuers sondados em ordem) por um caminho único de retomada: um índice de run suspenso por `(resourceID, threadID)` e um dispatcher que resolve o workflow dono e chama `Engine.Resume(def, key, mergePatch)` via `agent.WorkflowRegistry`. As chaves de correlação passam a ser uniformes `(resourceId, threadId, workflowId)`. Conforme ADR-002.

<requirements>
- RF-08: retomar o run suspenso correto por `(resource, thread)` sem cadeia ordenada de continuers.
- ADR-002: lookup único do run suspenso + despacho via `WorkflowRegistry`; substitui `tryResumeChain`.
- Invariante RF-09: no máximo 1 run suspenso por thread (a pendência ativa bloqueia novo workflow).
- Dependência: task 5.0 (workflow `transaction-write`) e task 6.0 (`budget-manage`, `card-manage`, `goal-edit`, `destructive-confirm`) para haver `Definition[S]` a registrar e resolver.
</requirements>

## Subtarefas

- [ ] 7.1 Uniformizar a chave de correlação `Key(resourceID, threadID, workflowID)` em todos os workflows novos (garantir que todos incluam sempre `(resourceId, threadId)`, hoje só pending-entry incluía `threadID`).
- [ ] 7.2 Implementar `SuspendedRunIndex.Resolve(resourceID, threadID) (workflowID, ok)` sobre o workflow store, retornando o workflow durável suspenso para a thread numa única consulta.
- [ ] 7.3 Implementar `resumeDispatcher` que resolve o run suspenso via `SuspendedRunIndex`, obtém a `Definition[S]` no `agent.WorkflowRegistry` e despacha `Engine.Resume(def, key, mergePatch)` do workflow dono.
- [ ] 7.4 Registrar as `Definition[S]` dos workflows novos no `WorkflowRegistry`.
- [ ] 7.5 Modificar `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go`: remover `tryResumeChain` e os 5 wrappers de continuer, ligando o dispatcher único.
- [ ] 7.6 Testes unitários do dispatcher + integração de suspend/resume (incluindo desambiguação e invariante de no máximo 1 run suspenso por thread).

## Detalhes de Implementação

Ver `techspec.md` (RF-08, seção "Remoção Total do Legado" — item `whatsapp_inbound_consumer.go` e continuers substituídos pelo resume dispatcher) e `adr-002-resume-routing-registry.md` desta pasta — **referenciar em vez de duplicar**.

Pontos-chave do ADR-002:
- Caminho de resume único e extensível: adicionar workflow não toca o consumer; resolução por `WorkflowRegistry` (aderência a R-AGENT-WF-001).
- Chave de correlação uniforme `(resourceId, threadId, workflowId)` habilita o índice.
- Resume por merge-patch sobre o `Snapshot.State` (contrato do kernel, R-WF-KERNEL-001.7); o dispatcher não faz parse antes do resume.
- Risco de mais de um run suspenso por thread mitigado por RF-09 (pendência ativa bloqueia novo workflow); coberto por teste de invariante. Rollback: reintroduzir a chain é local ao consumer.

## Critérios de Sucesso

- Run correto retomado por `(resource, thread)` sem `tryResumeChain`.
- Ponto único de despacho de resume via `WorkflowRegistry`.
- Invariante de no máximo 1 run suspenso por thread (RF-09) coberto por teste.
- Adicionar um workflow não exige tocar o consumer (registro no registry basta).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — roteamento por WorkflowRegistry e resume por merge-patch no substrato de agente.
- `design-patterns-mandatory` — gate de desenho do dispatcher e do índice de runs suspensos.

## Testes da Tarefa

- [ ] Testes unitários (dispatcher: resolução, desambiguação, ausência de run suspenso)
- [ ] Testes de integração (suspend/resume por `(resource, thread)`; invariante de 1 run suspenso por thread)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/usecases/` (novo `SuspendedRunIndex` e `resumeDispatcher`, ou pacote adequado)
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go`
- `internal/agents/module.go`
</content>
</invoke>
