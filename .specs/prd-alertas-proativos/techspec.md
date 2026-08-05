<!-- spec-hash-prd: 0c576e503ceb25e4988cef6a1cb2fb50c9964f3e4efca6d5e6943ecbc4c11d65 -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica: Alertas Proativos MeControla

## Resumo Executivo

A solução avança por fatias implementáveis e verificáveis. O primeiro corte entrega dry-run obrigatório, contrato de template genérico, envio por template aprovado, política de prioridade/supressão e follow-up agentivo mínimo para alertas Release 1. Nenhum envio real é permitido para template Meta sem status `APPROVED`.

O fluxo atual de budgets já possui job de thresholds, publisher outbox, consumer notifier e client Meta com `SendTemplate`. A implementação deve evoluir esses pontos sem quebrar `SendText`, `SendActivationTemplate`, outreach de ativação, WhatsApp inbound ou runtime agentivo.

## Arquitetura do Sistema

### Visão Geral dos Componentes

- `configs.BudgetsConfig`: adiciona dry-run, valida modo, quiet hours e flags de template.
- `internal/budgets/domain/services`: concentra decisão pura de prioridade, supressão e elegibilidade Release 1.
- `internal/budgets/application/usecases/EvaluateThresholdAlerts`: bloqueia efeitos colaterais quando dry-run estiver ativo.
- `internal/budgets/application/usecases/NotifyThresholdAlert`: envia template aprovado e só marca sucesso após aceite do gateway.
- `internal/platform/notification`: adiciona `TemplateMessage` tipado e envio genérico preservando métodos atuais.
- `internal/platform/notification/adapters`: adapta WhatsApp concreto ao contrato compartilhado sem regra financeira.
- `internal/onboarding/infrastructure/gateway/WhatsAppGateway`: traduz `TemplateMessage` para componentes Meta e delega para `meta.Client.SendTemplate`.
- `internal/onboarding/infrastructure/http/client/meta.Client`: permanece como client HTTP Meta.
- `internal/agents`: usa contexto de alerta recente no inbound para follow-up, consumindo primitives existentes.

Fluxo Release 1:

```text
Worker cron
  -> EvaluateThresholdAlerts
     -> DecideThresholdProactiveAlerts
     -> ApplyDailyPriority
     -> SuppressQuietHours/OptIn/Template
     -> se dry-run: metricar/logar e encerrar sem outbox/dedup
     -> se envio habilitado: publicar outbox e registrar dedup
        -> threshold notifier
           -> resolver canal
           -> enviar template aprovado
           -> marcar notificado somente após sucesso
           -> persistir contexto de follow-up

WhatsApp inbound
  -> dispatcher existente
  -> AgentRuntime existente
  -> resolver contexto recente de alerta
  -> escolher ferramenta existente ou pedir esclarecimento
```

## Design de Implementação

### Interfaces Chave

REQ-01: `notification.ChannelGateway` deve preservar compatibilidade:

```go
type ChannelGateway interface {
    SendText(ctx context.Context, channel, externalID, text string) error
    SendActivationTemplate(ctx context.Context, channel, externalID, templateName, token string) (string, error)
    SendTemplate(ctx context.Context, message TemplateMessage) (string, error)
}
```

REQ-02: `TemplateMessage` deve ter campos tipados: `Channel`, `ExternalID`, `TemplateName`, `LanguageCode`, `Components`.

REQ-03: componentes devem ser tipos fechados para `body`, `header` e `button`; parâmetros Release 1 usam `text`.

REQ-04: `TemplateMessage.Validate()` deve bloquear canal vazio, destinatário vazio, template vazio, componente sem tipo e parâmetro textual vazio.

REQ-05: `SendActivationTemplate` deve continuar como wrapper compatível para o template atual de ativação.

REQ-06: O gateway WhatsApp concreto deve transformar `TemplateMessage` em `components` Meta e delegar para `meta.Client.SendTemplate`.

### Modelos de Domínio e Aplicação

REQ-07: Alertas Release 1 devem usar tipos fechados para kind: `category_threshold_80`, `category_threshold_100`, `budget_missing_month_start`, `budget_not_reviewed_day_3`.

REQ-08: `category_threshold_90` deve existir apenas como estado bloqueado/configurado para futuro, sem emissão no Release 1.

REQ-09: Política de prioridade Release 1: `category_threshold_100`, `budget_not_reviewed_day_3`, `budget_missing_month_start`, `category_threshold_80`.

REQ-10: Supressão diária deve permitir no máximo um alerta iniciado pelo sistema por usuário por rodada.

REQ-11: Quiet hours deve bloquear envio entre 20:00 e 08:00 no timezone do usuário; fallback `America/Sao_Paulo`.

REQ-12: Templates `MARKETING` exigem opt-in explícito antes de avaliação de envio.

REQ-13: Dry-run deve encerrar antes de publicar outbox, inserir dedup ou chamar Meta.

REQ-14: Falha de envio Meta não pode marcar alerta como notificado.

REQ-15: Contexto de follow-up deve expirar; resposta curta sem contexto válido deve pedir esclarecimento.

### Persistência

REQ-16: Não criar tabela genérica no Release 1. A primeira entrega usa o fluxo existente de thresholds e não amplia persistência para todos os alertas.

REQ-17: Qualquer migration futura para 90% ou `proactive_alerts` exige `postgresql-production-standards`, rollback e testes de constraints.

REQ-18: Dedup Release 1 deve continuar impedindo repetição por usuário, orçamento/categoria, kind e período, sem colapsar 80 e 100 na mesma chave.

### Configuração

REQ-19: Adicionar configuração de dry-run com default seguro para produção.

REQ-20: Validar modo inválido de threshold alerts para não desligar fluxos silenciosamente.

REQ-21: Adicionar flags de template por kind Release 1 e validar que envio real exige template configurado e aprovado.

REQ-22: Adicionar config de quiet hours: start `20:00`, end `08:00`, timezone fallback `America/Sao_Paulo`.

### Endpoints de API

Não há endpoint HTTP novo. O acionamento é por worker/job e WhatsApp inbound existente.

## Pontos de Integração

- Meta WhatsApp Cloud API: envio por template aprovado via client existente.
- Outbox interno: envio real continua assíncrono e idempotente.
- AgentRuntime: follow-up usa Thread/Run/WorkingMemory/MessageStore existentes.

Tratamento de erro:

- 4xx Meta vira erro de client e não marca sucesso.
- 5xx Meta permite retry conforme política existente.
- Template ausente, pendente, rejeitado ou não configurado suprime envio com motivo.
- Dry-run não chama Meta.

## Abordagem de Testes

### Testes Unitários

- `internal/platform/notification`: validação de template, canal desconhecido, canal sem suporte, dispatch correto e preservação dos métodos antigos.
- `internal/platform/notification/adapters`: bridge WhatsApp para texto, ativação e template genérico.
- `internal/onboarding/infrastructure/gateway`: payload Meta para template genérico e ativação existente.
- `internal/budgets/domain/services`: prioridade, supressão diária, quiet hours, opt-in e bloqueio de 90%.
- `internal/budgets/application/usecases`: dry-run sem outbox/dedup/Meta; envio real preservado.
- `internal/agents`: follow-up com contexto válido/expirado.

### Testes de Integração

Integration tests são obrigatórios para o fluxo budgets/outbox afetado, usando os padrões existentes do módulo. Contract tests HTTP com `httptest` bastam para Meta na primeira entrega.

### Testes E2E

Envio real Meta não roda em CI padrão. Deve existir checklist operacional para validar em ambiente controlado com template `APPROVED`.

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. Fechar config e dry-run de thresholds.
2. Implementar política de domínio Release 1.
3. Evoluir contrato de template genérico.
4. Enviar thresholds por template aprovado sem marcar sucesso antes do envio.
5. Integrar contexto de follow-up ao inbound agentivo.
6. Adicionar observabilidade e gates de rollout.
7. Validar Release 1 em dry-run antes de desligar o bloqueio de envio real.

### Dependências Técnicas

Não há decisão de produto em aberto. Existem gates objetivos:

- Envio real de cada alerta exige template Meta `APPROVED`.
- Threshold 90% exige migration/modelagem futura.
- Meta atingida exige entidade de meta individual futura.

## Monitoramento e Observabilidade

REQ-23: Métricas com labels de baixa cardinalidade:

- `proactive_alerts_evaluated_total{kind,outcome}`
- `proactive_alerts_suppressed_total{kind,reason}`
- `proactive_alerts_queued_total{kind}`
- `proactive_alerts_notified_total{kind,channel,outcome}`
- `proactive_alerts_template_status{kind,status}`
- `proactive_alerts_dry_run_total{kind,outcome}`

REQ-24: Logs não podem registrar telefone completo, token Meta, access token, payload financeiro sensível ou erro bruto com segredo.

REQ-25: Runbook deve exigir consulta de status Meta antes de habilitar envio real por kind.

## Considerações Técnicas

### Decisões Chave

- ADR-001: Release 1 usa dry-run antes de envio real.
- ADR-002: Não aplicar design pattern formal; usar evolução direta de contrato e adapter fino.
- ADR-003: Threshold 90% fica condicionado a migration e modelagem específica.

### Alternativas Rejeitadas

- Enviar todos os alertas elegíveis no mesmo dia: rejeitado por risco de spam, custo e qualidade WhatsApp.
- Enfileirar alertas espaçados no Release 1: rejeitado por adicionar complexidade operacional antes do dry-run.
- Usar texto livre como fallback de template pendente: rejeitado por não ser production-ready fora da janela WhatsApp.
- Criar tabela `proactive_alerts` no Release 1: rejeitado por ampliar escopo antes de validar thresholds e template genérico.
- Usar categoria orçamentária `Metas` como meta individual: rejeitado por falso positivo de domínio.

### Riscos Conhecidos Controlados

- Templates Meta pendentes bloqueiam envio real daquele kind por gate objetivo.
- `ChannelGateway` é contrato compartilhado; toda implementação fake/stub precisa ser atualizada na tarefa correspondente.
- O notifier atual marca notificado antes do envio; a tarefa de template deve corrigir isso para não criar sucesso falso.

### Conformidade com Padrões

- `go-implementation`: mudança transversal com validação proporcional ampla.
- `mastra`: follow-up consome `AgentRuntime`, Thread, Run e WorkingMemory existentes.
- `domain-modeling-production`: estados de alerta, dedup e supressão como tipos fechados.
- `design-patterns-mandatory`: seletor retornou `reject`; decisão é `não aplicar padrão`.
- `postgresql-production-standards`: obrigatório apenas quando houver migration futura.

### Arquivos Relevantes e Dependentes

- `configs/config.go`
- `configs/config_test.go`
- `cmd/worker/worker.go`
- `internal/bootstrap/channel.go`
- `internal/platform/notification/channel.go`
- `internal/platform/notification/channel_test.go`
- `internal/platform/notification/adapters/whatsapp.go`
- `internal/platform/notification/adapters/adapters_test.go`
- `internal/onboarding/infrastructure/gateway/whatsapp_gateway.go`
- `internal/onboarding/infrastructure/gateway/whatsapp_gateway_test.go`
- `internal/onboarding/infrastructure/http/client/meta/client.go`
- `internal/onboarding/infrastructure/http/client/meta/client_test.go`
- `internal/budgets/application/usecases/evaluate_threshold_alerts.go`
- `internal/budgets/application/usecases/evaluate_threshold_alerts_test.go`
- `internal/budgets/application/usecases/notify_threshold_alert.go`
- `internal/budgets/application/usecases/notify_threshold_alert_test.go`
- `internal/budgets/domain/services/threshold_workflow.go`
- `internal/budgets/infrastructure/jobs/handlers/threshold_alerts_job.go`
- `internal/budgets/infrastructure/messaging/database/producers/threshold_alert_publisher.go`
- `internal/agents/application/agents/mecontrola_agent.go`
