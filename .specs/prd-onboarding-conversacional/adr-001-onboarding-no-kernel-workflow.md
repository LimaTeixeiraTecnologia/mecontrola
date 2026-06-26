# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Onboarding como workflow durável no kernel (8 etapas, substituição completa)
- **Data:** 2026-06-25
- **Status:** Aceita
- **Decisores:** JailtonJunior94 (product owner), time de plataforma
- **Relacionados:** PRD `.specs/prd-onboarding-conversacional/prd.md` (RF-19/22/23/29), techspec `techspec.md`, mapeamento `mapeamento-verbatim-onboarding.md`, R-AGENT-WF-001, R-WF-KERNEL-001

## Contexto

O onboarding atual roda em `internal/agent/application/usecases/run_onboarding_turn.go` como um loop de **5 fases** com headers "Etapa X/4", passo extra `first_tx`, auto-sugestão de split e conclusão travada na primeira transação. Isso **diverge das 8 etapas oficiais** (Cap. 07/08), usa `phase` como `string` livre (viola DMMF state-as-type) e contradiz o Cap. 07 (conclusão antes da operação diária). O repositório já possui um kernel genérico de workflow durável (`internal/platform/workflow`, `Engine[S]`, suspend/resume, merge-patch) preparado para fluxos conversacionais multi-turno.

## Decisão

Remodelar o onboarding como um **workflow durável de 8 etapas** sobre `Engine[OnboardingState]`, com **suspend/resume por etapa** e `Run` auditável. **Substituir por completo** o loop atual: remover `run_onboarding_turn.go`, `OnbPhaseFirstTx`/`firstTxPhase`, `buildAutoSplitPreview`/`suggest_budget_split` do caminho oficial, headers "Etapa X/4" e o schema `onboarding_first_tx`. A conclusão passa a ocorrer na ETAPA 8 após a confirmação do Resumo, **sem exigir primeira transação** (`IsReadyToComplete` sem `FirstTxRecorded`). O `internal/agent` é o consumidor do kernel; cada etapa é um `Step` que chama bindings→use cases do `internal/onboarding`. LLM apenas na cadeia de onboarding (exceção sancionada R-AGENT-WF-001.4); nada de domínio/LLM/SQL no kernel (R-WF-KERNEL-001).

## Alternativas Consideradas

- **Incremental com feature flag (coexistência).** Vantagem: menor risco de regressão. Desvantagem: dualidade de código, manutenção de dois fluxos, dívida temporária. Rejeitada por contrariar "sem flexibilizar" e o hábito do repositório de remover legado morto; o ganho de risco não compensa a complexidade prolongada pré-escala.
- **Manter o loop atual e só tipar a fase + fechar gaps.** Vantagem: menor esforço. Desvantagem: mantém modelo de 4 etapas, sem suspend/resume auditável padronizado, e não resolve a divergência estrutural com o oficial. Rejeitada.

## Consequências

### Benefícios Esperados
- Fidelidade estrutural às 8 etapas oficiais; ordem e gates corretos.
- Estado durável e auditável por etapa (resume robusto, `Run` rastreável).
- Eliminação de legado morto e de auto-sugestão não-oficial; base evolutiva única.

### Trade-offs e Custos
- Reescrita do fluxo de condução do onboarding e do wiring do `OnboardingAgent`.
- Dupla fonte de estado (snapshot do kernel × `onboarding_sessions`) a ser disciplinada (ver techspec R3).

### Riscos e Mitigações
- **Regressão na troca:** mitigar com testes unit/integração/e2e cobrindo as 8 etapas e os casos de borda antes de remover o legado. Rollback: reverter o wiring para o fluxo antigo enquanto não houver tráfego real relevante.

## Plano de Implementação
1. Tipos fechados + `Decide*` puros. 2. Use cases/eventos onboarding. 3. Steps + `OnboardingWorkflow`. 4. Wiring `OnboardingAgent` (resume antes do parse). 5. Remoção do legado. 6. Testes integração/e2e.

## Monitoramento e Validação
- `onboarding_completed_total`, `onboarding_run_duration_seconds`, `onboarding_step_total{step,outcome}`.
- Critério de sucesso: jornada completa das 8 etapas passando em e2e; 0 referência a "Etapa X/4"/`first_tx` no código.

## Impacto em Documentação e Operação
- Atualizar runbook de jornada do onboarding (diálogo verbatim por etapa).
- Dashboards Grafana de funil de onboarding.

## Revisão Futura
- Revisar se surgir necessidade de ramificações não-lineares ou multicanal; ou se o kernel evoluir o contrato de suspend/resume.
