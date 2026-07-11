# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Rollout sem feature flag, allowlist ou canary
- **Data:** 2026-07-11
- **Status:** Aceita
- **Decisores:** Requester, Codex
- **Relacionados:** `prd.md`, `techspec.md`

## Contexto

Durante a clarificação, o requester rejeitou explicitamente feature flag. A validação pós-deploy deve ser feita diretamente com o usuário de teste do requester, mas o release não terá isolamento por allowlist ou canary. Isso altera a estratégia operacional: a qualidade precisa ser fechada por testes, golden/eval, integração, carga e rollback por deploy reversal.

## Decisão

A funcionalidade será entregue sem feature flag, sem allowlist e sem canary. O deploy será geral para produção. A validação manual pós-deploy usará apenas o usuário de teste do requester como jornada observada, mas qualquer usuário elegível poderá passar pelo novo comportamento após o release.

Rollback será reversão de deploy, não desligamento seletivo.

## Alternativas Consideradas

- Feature flag por usuário: reduziria risco de exposição, mas foi rejeitada pelo requester.
- Allowlist do usuário de teste: permitiria validação isolada, mas também foi rejeitada pela restrição de não usar flag.
- Canary por percentual: reduziria blast radius, mas adicionaria infraestrutura operacional fora da decisão confirmada.

## Consequências

### Benefícios Esperados

- Menor complexidade de implementação.
- Sem caminhos condicionais em produção.
- Testes representam exatamente o comportamento que será publicado.

### Trade-offs e Custos

- Blast radius é produção inteira no momento do deploy.
- Rollback exige reversão de versão.
- Não há kill switch específico para esta feature.

### Riscos e Mitigações

- Risco: regressão impactar outros usuários recém-ativados.
- Mitigação: gates completos antes do deploy e validação manual imediata.

- Risco: falso sucesso financeiro em produção.
- Mitigação: alerta crítico de confirmação positiva sem transação ativa em até 30s.

## Plano de Implementação

1. Não introduzir feature flag nem branches condicionais por usuário.
2. Executar testes unitários, integração, golden/eval e carga antes do deploy.
3. Confirmar configs de produção: `TRANSACTIONS_ENABLED`, `OUTBOX_DISPATCHER_ENABLED`, OpenRouter e timeouts.
4. Publicar release geral.
5. Executar jornada manual com o usuário de teste do requester.
6. Reverter deploy se qualquer critério crítico falhar.

## Monitoramento e Validação

- Monitorar ativação até primeira transação ativa.
- Monitorar dead-letter/outbox e runs falhos.
- Validar manualmente mensagens, workflow, outbox e `transactions`.

## Impacto em Documentação e Operação

- Runbook deve explicitar que não há flag de desligamento.
- Checklist de deploy deve incluir comando/procedimento de reversão.

## Revisão Futura

Revisar a decisão se o produto exigir rollout gradual para jornadas agentivas futuras.
