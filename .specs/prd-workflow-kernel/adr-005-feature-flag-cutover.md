# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Cutover do write de transactions por feature flag com fallback ao caminho atual
- **Data:** 2026-06-24
- **Status:** Aceita
- **Decisores:** Plataforma, dono do `internal/agent`
- **Relacionados:** PRD (RF-17, RF-19, RF-24 — migração aditiva, não regressão); techspec (Sequenciamento; Resume); ADR-001; ADR-003

## Contexto

- A migração é **aditiva e incremental**: o write de transactions é o fluxo de prova, e o PRD exige
  **zero regressão** comprovável ("production-proof, sem falso positivo").
- O caminho atual (`IntentRegistry` + `WriteGuard` + `pendingexpense` em `agent_sessions`) está em
  produção e atende usuários reais via WhatsApp/Telegram.
- Uma troca direta no mesmo PR não oferece rollback rápido caso um caso de borda escape à suíte.

## Decisão

- Introduzir a feature flag `WORKFLOW_KERNEL_TRANSACTIONS_WRITE_ENABLED` (env, default **false**).
  - **Off (default):** `dispatchWrite` e `continuePendingExpenseConfirmation` usam o caminho atual,
    **inalterado**.
  - **On:** delegam ao `Engine.Start`/`Engine.Resume` do kernel (Definition `transactions_write`),
    com fallback de leitura do draft legado em `agent_sessions` durante a janela de drenagem (ADR-003).
- O caminho atual permanece no código durante a transição (coexistência temporária), permitindo
  **rollback instantâneo** por env, sem deploy.
- A suíte de **não regressão** dirigida por tabela compara reply/outcome/kind dos dois caminhos para os
  mesmos inputs (auto-log, ambiguous→choice→resume, needs_confirm→confirm/cancel, authz_denied,
  replay, policy_blocked, usecase_error, missing_resolver).

## Alternativas Consideradas

- **Cutover direto no mesmo PR**: menos código morto, mas sem rollback rápido em produção; rejeitada
  pelo requisito de zero regressão production-proof.
- **Coexistência permanente (kernel só p/ kinds novos)**: não migra transactions write; contraria a
  prova de migração do PRD (RF-22/23); rejeitada.

## Consequências

### Benefícios Esperados

- Rollback instantâneo por env; risco operacional minimizado.
- Validação A/B do comportamento (paridade) antes de remover o caminho antigo.

### Trade-offs e Custos

- Dois caminhos coexistem temporariamente (custo de manutenção e de teste duplo).
- Necessidade de uma decisão futura de remoção do caminho antigo.

### Riscos e Mitigações

- **Risco:** divergência sutil entre caminhos. **Mitigação:** suíte de paridade obrigatória (RF-24);
  flag default off até a paridade estar verde.
- **Risco:** código morto persistir. **Mitigação:** ADR de remoção agendada após estabilização.
- **Rollback:** desligar a flag.

## Plano de Implementação

1. Config + validação da flag (`configs/config.go` + testes).
2. Branch de delegação em `dispatchWrite`/`continuePendingExpenseConfirmation` sob flag.
3. Suíte de não regressão verde com flag on; manter default off no merge.
4. Conclusão: paridade comprovada; flag pronta para ativação controlada por ambiente.

## Monitoramento e Validação

- Comparar `agent_*` (caminho atual) e `workflow_*` (kernel) por ambiente durante o ramp.
- Sucesso: 0 divergência de reply/outcome; métricas de erro do kernel ≤ baseline do caminho atual.
- Reverter (flag off) ao primeiro sinal de regressão; investigar; reativar.

## Impacto em Documentação e Operação

- `.env`/configs documentando a flag.
- Runbook: procedimento de ativação/rollback e leitura das métricas de paridade.

## Revisão Futura

- **Decisão agendada:** após estabilização em produção (a definir na ativação), abrir ADR para
  **remover o caminho antigo** e encerrar a janela de drenagem do draft legado, migrando os demais
  kinds de escrita ao kernel no mesmo padrão.
