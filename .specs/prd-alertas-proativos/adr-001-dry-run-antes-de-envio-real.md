# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Dry-run antes de envio real
- **Data:** 2026-08-05
- **Status:** Aceita
- **Decisores:** Produto e engenharia MeControla
- **Relacionados:** `.specs/prd-alertas-proativos/prd.md`, `.specs/prd-alertas-proativos/techspec.md`, `docs/refin/2026-08-05-sdd-alertas-proativos.md`

## Contexto

Alertas proativos podem gerar mensagens fora da janela WhatsApp e afetar custo, reputação do número, experiência do usuário e confiança nos dados financeiros. O fluxo atual de thresholds já publica outbox e grava dedup quando o job roda. Sem dry-run, a primeira ativação do job pode produzir efeitos colaterais antes de medir volume e qualidade.

## Decisão

O Release 1 deve iniciar com dry-run bloqueando efeitos antes de publicação de outbox, marcação de dedup, marcação de notificação e chamada à Meta.

## Alternativas Consideradas

- Enviar direto com feature flag: rejeitada porque não mede volume antes do efeito externo.
- Fazer dry-run no notifier: rejeitada porque ainda criaria outbox e poderia marcar estados intermediários.
- Usar ambiente separado apenas: rejeitada porque não mede comportamento real da base produtiva.

## Consequências

### Benefícios Esperados

- Reduz risco de spam e custo inesperado.
- Permite medir elegibilidade, prioridade e supressão.
- Mantém produção sem envio real até templates e políticas estarem prontos.

### Trade-offs e Custos

- Exige instrumentação suficiente para observar candidatos.
- Atrasa envio real até haver evidência operacional.

### Riscos e Mitigações

- Risco: dry-run ficar ligado por engano. Mitigação: logs e métricas explícitas de modo.
- Risco: falso senso de prontidão. Mitigação: gates exigem template `APPROVED` e teste de envio antes de produção.

## Plano de Implementação

1. Adicionar flag de dry-run em budgets threshold alerts.
2. Encerrar `EvaluateThresholdAlerts` antes de outbox/dedup quando dry-run estiver ativo.
3. Cobrir com testes unitários.
4. Rodar em produção com modo `both` e dry-run antes do envio real.

## Monitoramento e Validação

- Métrica de alertas avaliados por `kind` e `outcome`.
- Métrica de dry-run por `kind`.
- Logs sem dados sensíveis com modo e contagem agregada.

## Impacto em Documentação e Operação

- Atualizar runbook de rollout.
- Registrar templates aprovados antes de desligar dry-run.

## Revisão Futura

Revisar quando templates Release 1 estiverem `APPROVED` e houver pelo menos um ciclo completo de dry-run produtivo.
