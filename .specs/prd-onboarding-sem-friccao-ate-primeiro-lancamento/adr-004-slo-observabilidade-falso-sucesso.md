# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** SLO e observabilidade contra falso sucesso financeiro
- **Data:** 2026-07-11
- **Status:** Aceita
- **Decisores:** Requester, Codex
- **Relacionados:** `prd.md`, `techspec.md`

## Contexto

A falha original não foi apenas conversacional: o sistema concluiu onboarding e depois não persistiu lançamento financeiro ativo. O PRD exige 0 falso sucesso. Portanto, validar apenas resposta do agente ou status do workflow é insuficiente; a evidência final precisa chegar em `transactions` com confirmação humana e origem rastreável.

## Decisão

O critério operacional principal será ativação até primeira transação ativa. O SLO técnico é: 95% das jornadas cobertas devem sair da ativação até primeira transação ativa em até 5 minutos, excluindo tempo de espera do usuário. Confirmação positiva sem transação ativa em até 30s é alerta crítico.

Métricas, logs e traces devem manter cardinalidade controlada e não podem expor telefone, `user_id`, `wamid`, categoria, descrição financeira ou IDs de entidade como labels.

## Alternativas Consideradas

- Medir apenas mensagem enviada: barato, mas aceita falso sucesso.
- Medir apenas workflow concluído: cobre onboarding, mas não primeiro lançamento.
- Medir apenas tool success: pode ignorar falha posterior na escrita ou outbox.

## Consequências

### Benefícios Esperados

- Detecção direta de falso sucesso financeiro.
- Evidência operacional fim a fim.
- Redução de regressões silenciosas em produção.

### Trade-offs e Custos

- Requer consulta/alerta correlacionando confirmação e escrita ativa.
- Pode exigir ajuste em dashboards ou runbooks existentes.
- SLO exclui espera do usuário, o que precisa estar claro na análise.

### Riscos e Mitigações

- Risco: alerta com falso positivo por atraso temporário de outbox ou banco.
- Mitigação: janela de 30s e validação por status ativo da transação.

- Risco: métrica vazar dado sensível.
- Mitigação: labels permitidos apenas por workflow, step, status, outcome, guard e agent_id controlado.

## Plano de Implementação

1. Reaproveitar métricas de workflow, agent, guard, outbox e transactions sempre que suficientes.
2. Adicionar alerta/runbook para confirmação positiva sem transação ativa.
3. Atualizar pós-deploy para verificar `workflow_runs`, `workflow_steps`, `platform_messages`, `outbox_events`, `platform_runs` e `transactions`.
4. Adicionar golden/eval para impedir falso multi-lançamento e falso sucesso verbal.

## Monitoramento e Validação

- SLO: 95% ativação até primeira transação ativa em até 5 minutos, excluindo espera do usuário.
- Crítico: confirmação positiva sem transação ativa em até 30s.
- Webhook inbound p95 < 500ms em carga.
- Custo por jornada não pode exceder baseline de staging em mais de 30%.

## Impacto em Documentação e Operação

- Atualizar alertas e runbook pós-deploy.
- Registrar consultas ou dashboard necessários para comprovar primeira transação ativa.

## Revisão Futura

Revisar após o primeiro release em produção ou se surgirem jornadas financeiras além de pix e receita simples.
