# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Roteamento de resume por lookup único do run suspenso + WorkflowRegistry
- **Data:** 2026-07-15
- **Status:** Aceita
- **Decisores:** Solicitante do produto, engenharia de plataforma
- **Relacionados:** PRD `prd.md`; techspec `techspec.md`; R-AGENT-WF-001 (roteamento por registry)

## Contexto

Hoje a retomada de um workflow suspenso é resolvida por `tryResumeChain`, um slice hardcoded de 5 continuers testados em ordem; cada continuer sonda cegamente seu próprio `store.Load(workflowID, key)` e a única desambiguação é a ordem do slice. Não há registry central de runs suspensos por `(resource, thread)`; cada workflow tem struct de continuer, assinatura de `Continue` e convenção de chave distintas (só pending-entry inclui `threadID`). Adicionar um workflow exige tocar interface, campo, option e o slice do consumer. Isso viola o espírito de R-AGENT-WF-001 (resolução por registry) e cresce em acoplamento a cada workflow.

## Decisão

Introduzir um **lookup único do run suspenso por `(resourceID, threadID)`** e despachar a retomada via `agent.WorkflowRegistry`. O consumer resolve, numa consulta, qual workflow durável está suspenso para a thread e chama `Engine.Resume(def, key, mergePatch)` do workflow dono. As chaves de correlação passam a incluir sempre `(resourceId, threadId)` para o índice ser uniforme. Substitui `tryResumeChain`.

## Alternativas Consideradas

- **Manter a cadeia ordenada de continuers**: menor mudança imediata; rejeitada por perpetuar acoplamento, ordem implícita e uma struct/assinatura por workflow, dificultando a extensão exigida pelos fluxos novos.

## Consequências

### Benefícios Esperados

- Caminho de resume único e extensível; adicionar workflow não toca o consumer.
- Aderência a R-AGENT-WF-001 (registry) e chave de correlação uniforme.

### Trade-offs e Custos

- Exige uniformizar as chaves de correlação e um índice/consulta de run suspenso por thread.

### Riscos e Mitigações

- Risco: mais de um run suspenso na mesma thread. Mitigação: a pendência ativa bloqueia novo workflow (RF-09), garantindo no máximo um suspenso por thread; teste de invariante cobre. Rollback: reintroduzir a chain é local ao consumer.

## Plano de Implementação

1. Uniformizar `Key(resourceID, threadID, workflowID)` em todos os workflows.
2. Implementar `SuspendedRunIndex.Resolve(resourceID, threadID) (workflowID, ok)` sobre o workflow store.
3. Registrar as `Definition[S]` no `WorkflowRegistry`; `resumeDispatcher` despacha o `Resume`.
4. Remover `tryResumeChain` e os 5 continuers hardcoded.

## Monitoramento e Validação

- Métrica de resume por `workflow`/`outcome`; testes unitários do dispatcher; integração de suspend/resume.
- Sucesso: retomada correta sem chain, com um único ponto de despacho.

## Impacto em Documentação e Operação

- Atualizar runbook de roteamento inbound e o diagrama Thread->Run.

## Revisão Futura

- Revisitar se surgir necessidade legítima de múltiplos runs suspensos concorrentes por thread.
