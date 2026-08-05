# SDD: Alertas Proativos MeControla

Data: 2026-08-05

Status: documento unificado, canonico e fechado para desenvolvimento do Release 1. Nao ha decisao de produto em aberto; gates externos de envio e releases posteriores estao explicitados como pre-condicoes objetivas.

Fontes:

- Documento de negocio: `/Users/jailtonjunior/Downloads/MeControla_Alertas_Proativos_Documentacao_Final.md`
- Apêndice operacional de templates: `docs/refin/2026-08-05-meta-templates-alertas-proativos.md`
- Codebase local em `/Users/jailtonjunior/Git/mecontrola`
- Consulta Meta Graph API feita com `.env` local, sem registrar segredos
- Documentacao oficial Meta: Message Templates, Message Template API, Template Components, Template Categorization e WhatsApp permissions

## 1. Objetivo

Projetar a capacidade de alertas proativos do MeControla para WhatsApp com:

- decisao deterministica de elegibilidade;
- envio idempotente;
- prioridade e supressao auditaveis;
- compatibilidade com Meta WhatsApp Cloud API;
- reuso do runtime agentivo atual para respostas do usuario;
- zero regressao nos fluxos existentes de onboarding, WhatsApp inbound, budgets threshold e agente financeiro.

## 2. Escopo

### 2.1 Incluso no desenho

- Alertas de categoria em 80% e 100% no Release 1.
- Alerta de categoria em 90% especificado como Release posterior condicionado a VO, constraint e migration.
- Alertas de orcamento inexistente no inicio do mes e nao revisado ate o terceiro dia.
- Fechamento do mes.
- Motivacao semanal.
- Retomada de uso apos 3 dias.
- Risco de abandono apos 7 dias.
- Modelo para meta atingida, condicionado a prova de meta individual.
- Especificacao de templates Meta.
- Plano de implementacao incremental.
- Gates de validacao.

### 2.2 Fora do primeiro release recomendado

- Meta atingida como meta individual, enquanto nao houver entidade/prova no codebase.
- Otimizacao de horario por comportamento do usuario.
- Experimentos A/B de texto.
- Multicanal alem de WhatsApp.
- Machine learning para churn.

## 3. Evidencias do codebase

- Config WhatsApp existe em `configs/config.go:153`.
- Producao valida credenciais Meta em `configs/config.go:1001`.
- BudgetsConfig possui cron, scan limit, modo e ratios de threshold em `configs/config.go:102`.
- Worker registra `ThresholdAlertsJob` quando o modulo o expoe em `cmd/worker/worker.go:389`.
- `ThresholdAlertsJob` roda por cron e delega para use case em `internal/budgets/infrastructure/jobs/handlers/threshold_alerts_job.go:29`.
- `EvaluateThresholdAlerts` decide e publica alertas em transacao em `internal/budgets/application/usecases/evaluate_threshold_alerts.go:76`.
- `ThresholdWorkflow` decide alertas por snapshot e dedup em `internal/budgets/domain/services/threshold_workflow.go:59`.
- `NotifyThresholdAlert` marca notificado antes de enviar texto livre em `internal/budgets/application/usecases/notify_threshold_alert.go:107`.
- `ChannelGateway` so tem `SendText` e `SendActivationTemplate` em `internal/platform/notification/channel.go:20`.
- Client Meta ja suporta template generico em `internal/onboarding/infrastructure/http/client/meta/client.go:75`.
- Gateway atual so expoe template de ativacao em `internal/onboarding/infrastructure/gateway/whatsapp_gateway.go:20`.
- Categorias oficiais estao fechadas em `internal/budgets/domain/valueobjects/root_slug.go:12`.
- Threshold atual so aceita 80 e 100 em `internal/budgets/domain/valueobjects/threshold.go:12`.
- Constraint historica `budgets_alerts_threshold_chk` aceita apenas 80 e 100 em `migrations/000001_initial_schema.up.sql:742`.
- Idempotencia atual `budget_alerts_sent` usa PK `(user_id, budget_id, kind, ref_day)` em `migrations/000001_initial_schema.up.sql:1075`.
- Agente ja possui ferramentas de orcamento e categoria em `internal/agents/application/agents/mecontrola_agent.go:184`.

## 4. Evidencia Meta

Consulta Meta em 2026-08-05:

- Numero: `+55 11 93621-2870`
- Verified name: `MeControla`
- Quality rating: `GREEN`
- Templates aprovados:
  - `mecontrola_ativacao`, `APPROVED`, `MARKETING`, `pt_BR`
  - `hello_world`, `APPROVED`, `UTILITY`, `en_US`

Templates de alertas proativos submetidos via Meta Graph API em 2026-08-05:

| AlertKind | Template | ID Meta | Categoria | Idioma | Status |
| --- | --- | --- | --- | --- | --- |
| `budget_missing_month_start` | `mecontrola_budget_missing_month_start` | `1561236175466476` | `UTILITY` | `pt_BR` | `PENDING` |
| `category_threshold_80` | `mecontrola_category_threshold_80` | `1710238193529722` | `UTILITY` | `pt_BR` | `PENDING` |
| `category_threshold_90` | `mecontrola_category_threshold_90` | `1062335899595161` | `UTILITY` | `pt_BR` | `PENDING` |
| `category_threshold_100` | `mecontrola_category_threshold_100` | `1376417771353202` | `UTILITY` | `pt_BR` | `PENDING` |
| `budget_not_reviewed_day_3` | `mecontrola_budget_not_reviewed_day_3` | `1053591550739935` | `UTILITY` | `pt_BR` | `PENDING` |
| `month_closing` | `mecontrola_month_closing` | `1059680423220718` | `UTILITY` | `pt_BR` | `PENDING` |
| `weekly_motivation` | `mecontrola_weekly_motivation` | `1970815953590167` | `MARKETING` | `pt_BR` | `APPROVED` |
| `usage_reactivation_3d` | `mecontrola_usage_reactivation_3d` | `1746833263117443` | `MARKETING` | `pt_BR` | `APPROVED` |
| `abandonment_risk_7d` | `mecontrola_abandonment_risk_7d` | `871497755818159` | `MARKETING` | `pt_BR` | `PENDING` |
| `goal_achieved` | `mecontrola_goal_achieved` | `892397930196406` | `UTILITY` | `pt_BR` | `PENDING` |

Observacoes de prova:

- A Meta API permite criar templates via `POST /{WABA-ID}/message_templates`; os templates ficam sujeitos a aprovacao antes de uso em producao.
- `mecontrola_category_threshold_80` foi ajustado na submissao para evitar erro de parametro da Meta: a pergunta final cadastrada usa "nessa categoria" em vez de repetir `{{1}}`.
- `mecontrola_goal_achieved` foi submetido como template, mas seu uso continua bloqueado ate existir prova de meta individual no dominio.
- O template transitorio `mecontrola_usage_reactivation_7d` criado como fallback foi removido via API e nao faz parte do entregavel.

Gate Meta: envio proativo fora da janela WhatsApp so pode usar templates com status `APPROVED`. Nesta consulta, apenas `weekly_motivation` e `usage_reactivation_3d` estao aprovados entre os alertas proativos.

## 5. Arquitetura alvo

```text
Worker cron
  -> ProactiveAlerts use case
     -> coleta usuarios elegiveis
     -> coleta snapshots financeiros
     -> DecideProactiveAlerts
     -> aplica prioridade e frequencia
     -> persiste dedup/supressao
     -> publica outbox
        -> consumer notifier
           -> resolve canal
           -> escolhe template aprovado
           -> envia Meta
           -> marca notificado ou falha

WhatsApp inbound de resposta
  -> dispatcher existente
  -> AgentRuntime existente
  -> contexto do alerta recente
  -> ferramenta adequada
```

## 6. Decisoes de desenho

### 6.1 Decisao deterministica

Elegibilidade, prioridade, dedup e supressao devem ficar fora do LLM.

Justificativa: o agente ja tem regra anti-simulacao para nao inventar dados financeiros. A decisao de disparo depende de persistencia, eventos e regras de frequencia.

### 6.2 Reuso do modulo budgets

Alertas financeiros ligados a orcamento devem continuar em `internal/budgets`.

Justificativa: o modulo ja contem thresholds, monthly summary, alert repository, threshold sent repository, job e consumer.

### 6.3 Canal como adapter fino

WhatsApp/Meta nao decide regra de negocio. O adapter recebe template, parametros e destinatario.

### 6.4 Nao aplicar pattern novo

O seletor de `design-patterns-mandatory` retornou `reject`. A solucao deve ser evolucao direta de contratos, use cases, eventos e repositories existentes.

## 7. Modelo de dominio

### 7.1 AlertKind

Tipo fechado:

- `budget_missing_month_start`
- `category_threshold_80`
- `category_threshold_90`
- `category_threshold_100`
- `goal_achieved`
- `month_closing`
- `weekly_motivation`
- `usage_reactivation_3d`
- `abandonment_risk_7d`
- `budget_not_reviewed_day_3`

### 7.2 AlertTarget

Variantes:

- `UserTarget`: usado para motivacao, retomada e abandono.
- `CompetenceTarget`: usado para inicio, nao revisado e fechamento.
- `CategoryTarget`: usado para 80/90/100.
- `GoalTarget`: usado para meta atingida.

Gate de escopo: `GoalTarget` fica fora do Release 1 e depende de entidade de meta individual em release posterior.

### 7.3 AlertDedupKey

Chaves canonicas:

- Categoria: `user_id + kind + root_slug + competence`
- Orcamento inicio: `user_id + budget_missing_month_start + competence`
- Orcamento nao revisado: `user_id + budget_not_reviewed_day_3 + competence`
- Fechamento: `user_id + month_closing + competence`
- Motivacao: `user_id + weekly_motivation + iso_week`
- Retomada: `user_id + usage_reactivation_3d + iso_week`
- Abandono: `user_id + abandonment_risk_7d + yyyy_mm`
- Meta: `user_id + goal_achieved + goal_id`

### 7.4 AlertState

Tipo fechado:

- `eligible`
- `suppressed_priority`
- `suppressed_frequency`
- `suppressed_no_channel`
- `suppressed_no_template`
- `queued`
- `notified`
- `failed`
- `responded`

Zero value invalido.

### 7.5 AlertPriority

Ordem de maior para menor:

1. `category_threshold_100`
2. `category_threshold_90`
3. `goal_achieved`
4. `budget_not_reviewed_day_3`
5. `budget_missing_month_start`
6. `month_closing`
7. `category_threshold_80`
8. `usage_reactivation_3d`
9. `abandonment_risk_7d`
10. `weekly_motivation`

### 7.6 AlertTemplate

Campos:

- `kind`
- `template_name`
- `language_code`
- `category`
- `parameters`
- `approval_status`
- `last_verified_at`

## 8. Regras e invariantes

- Nao enviar alerta sem usuario, kind, periodo e dedup key.
- Nao enviar alerta de categoria sem `root_slug`.
- Nao enviar alerta de categoria se nao houver orcamento planejado positivo.
- 80, 90 e 100 sao alertas independentes.
- Nao repetir alerta para a mesma dedup key.
- Nao enviar motivacao no mesmo dia de 90 ou 100.
- Nao enviar proativo para usuario sem assinatura ativa ate decisao contraria.
- Nao enviar fora da janela WhatsApp usando texto livre sem template aprovado.
- Falha de envio Meta nao pode ser marcada como notificada.
- Adapter de WhatsApp nao pode conter regra financeira.
- Resposta curta como "sim" so pode acionar follow-up se houver contexto de alerta recente e nao expirado.

## 9. Politica de elegibilidade por alerta

### 9.1 Inicio de mes

Elegivel quando nao existir orcamento cadastrado para a competencia vigente.

Dedup: uma vez por usuario por competencia.

Follow-up: iniciar `create_budget`.

### 9.2 Categoria 80%

Elegivel quando gasto realizado da categoria atingir pelo menos 80% do planejado.

Dedup: uma vez por categoria por competencia.

Follow-up: detalhamento da categoria por subcategoria. A ferramenta mais proxima no codebase e `category_detail`, mas o SDD exige confirmar se ela retorna quebra por subcategoria para esse caso.

### 9.3 Categoria 90%

Elegivel quando gasto realizado da categoria atingir pelo menos 90% do planejado.

Gate de escopo: threshold 90 nao entra no Release 1 porque nao existe no VO nem nas constraints historicas.

Follow-up: panorama completo por categoria.

### 9.4 Categoria 100%

Elegivel quando gasto realizado da categoria atingir ou ultrapassar 100% do planejado.

Dedup: uma vez por categoria por competencia.

Follow-up: panorama completo por categoria.

### 9.5 Meta atingida

Elegivel quando valor acumulado da meta for maior ou igual ao valor definido.

Gate de escopo: meta atingida nao entra no Release 1 porque nao ha prova neste refinamento de entidade de meta individual com `nome_meta`, `valor_meta` e `valor_acumulado`.

### 9.6 Fechamento do mes

Elegivel no ultimo dia do mes ou fechamento da competencia financeira do usuario.

Dedup: uma vez por competencia.

Follow-up: montar base do proximo orcamento, sem criar automaticamente.

### 9.7 Motivacao semanal

Elegivel uma vez por semana para usuario ativo com registro recente.

Suprimir se houver 90 ou 100 no mesmo dia.

### 9.8 Retomada de uso

Elegivel quando usuario estiver 3 dias sem registrar gastos ou interagir e assinatura estiver ativa.

Dedup: no maximo uma vez por semana.

### 9.9 Risco de abandono

Elegivel quando usuario estiver 7 dias ou mais sem interacao.

Dedup: no maximo uma vez por mes.

Premissa segura: exigir assinatura ativa ate decisao de negocio contraria.

### 9.10 Orcamento nao revisado

Elegivel se usuario continuar sem orcamento cadastrado/revisado ate o terceiro dia do mes.

Dedup: um reforco por competencia.

## 10. Politica de prioridade

Para production-ready no Release 1, a politica deve escolher no maximo um alerta iniciado pelo sistema por usuario por rodada diaria, aplicando a ordem definida neste SDD. Os demais alertas elegiveis na mesma rodada devem ser suprimidos com motivo auditavel `suppressed_priority`.

## 11. Contratos tecnicos

### 11.1 Notification

Evoluir sem quebrar chamadas existentes:

- manter `SendText`;
- manter `SendActivationTemplate`;
- adicionar envio generico de template com `TemplateMessage`.

Contrato sugerido:

```text
TemplateMessage
  Channel
  ExternalID
  TemplateName
  LanguageCode
  BodyParameters[]
  ButtonParameters[]
```

### 11.2 Meta adapter

Reusar `meta.Client.SendTemplate`.

O gateway deve apenas traduzir `TemplateMessage` para componentes Meta.

### 11.3 Budgets application

Adicionar ou evoluir use cases:

- `EvaluateProactiveAlerts`
- `NotifyProactiveAlert`
- `ResolveProactiveAlertFollowUp`

Se o escopo inicial for apenas thresholds, pode evoluir `EvaluateThresholdAlerts` de forma incremental, mas evitando acoplar os 10 alertas a um use case que nasceu para threshold.

### 11.4 Agent follow-up

O inbound WhatsApp deve continuar pelo dispatcher e `AgentRuntime`.

Para "sim", o agente precisa receber contexto de alerta recente, por WorkingMemory, MessageStore ou uma porta de consulta dedicada. Nao reimplementar Thread/Run fora de `internal/platform/{agent,memory}`.

## 12. Persistencia

### 12.1 Caminho recomendado

Criar tabela generica para alertas proativos se o release incluir alertas alem de thresholds:

```text
proactive_alerts
  id
  user_id
  kind
  target_kind
  target_key
  period_key
  state
  priority
  template_name
  channel
  external_message_id
  eligible_at
  queued_at
  notified_at
  failed_at
  failure_reason
  responded_at
```

Unique key:

```text
user_id, kind, target_key, period_key
```

Gate PostgreSQL: qualquer migration exige `postgresql-production-standards`.

### 12.2 Alternativa restrita

Se o primeiro release for somente thresholds, evoluir `budget_alerts_sent` para incluir threshold/root slug na chave ou separar kinds por threshold.

Risco: pode ficar inadequado para fechamento, retomada, abandono e motivacao.

## 13. Configuracao

Novas variaveis recomendadas:

- `PROACTIVE_ALERTS_ENABLED`
- `PROACTIVE_ALERTS_CRON`
- `PROACTIVE_ALERTS_SCAN_LIMIT`
- `PROACTIVE_ALERTS_MODE`
- `PROACTIVE_ALERTS_REQUIRE_TEMPLATE`
- `PROACTIVE_ALERTS_QUIET_HOURS_START`
- `PROACTIVE_ALERTS_QUIET_HOURS_END`
- `META_TEMPLATE_BUDGET_MISSING_MONTH_START`
- `META_TEMPLATE_CATEGORY_THRESHOLD_80`
- `META_TEMPLATE_CATEGORY_THRESHOLD_90`
- `META_TEMPLATE_CATEGORY_THRESHOLD_100`
- `META_TEMPLATE_MONTH_CLOSING`
- `META_TEMPLATE_WEEKLY_MOTIVATION`
- `META_TEMPLATE_USAGE_REACTIVATION_3D`
- `META_TEMPLATE_ABANDONMENT_RISK_7D`
- `META_TEMPLATE_BUDGET_NOT_REVIEWED_DAY_3`
- `META_TEMPLATE_GOAL_ACHIEVED`

Validacao production:

- se `PROACTIVE_ALERTS_ENABLED=true` e `PROACTIVE_ALERTS_REQUIRE_TEMPLATE=true`, todos os templates do escopo habilitado devem estar preenchidos e aprovados.

## 14. Observabilidade

Metricas com labels de baixa cardinalidade:

- `proactive_alerts_evaluated_total{kind,outcome}`
- `proactive_alerts_suppressed_total{kind,reason}`
- `proactive_alerts_queued_total{kind}`
- `proactive_alerts_notified_total{kind,channel,outcome}`
- `proactive_alerts_followup_total{kind,outcome}`
- `proactive_alerts_template_status{kind,status}`

Labels proibidas:

- `user_id`
- telefone
- `message_id`
- `target_key`
- texto de erro bruto

Logs:

- usar `run_id`/event id quando existir;
- nao logar telefone completo nem parametros financeiros sensiveis em texto livre.

## 15. Erros tipados

Erros de dominio:

- `ErrAlertAlreadySent`
- `ErrAlertSuppressedByPriority`
- `ErrAlertSuppressedByFrequency`
- `ErrAlertNoEligibleChannel`
- `ErrAlertTemplateMissing`
- `ErrAlertTemplateNotApproved`
- `ErrAlertTargetInvalid`
- `ErrAlertFollowUpContextExpired`

Erros de adapter:

- auth Meta;
- bad request Meta;
- rate limit Meta;
- server error Meta;
- timeout;
- template rejeitado/ausente.

## 16. Rollout

### 16.1 Release 0: sem envio real

- Implementar avaliacao e supressao em dry-run.
- Medir elegiveis por kind.
- Validar volume e prioridade.

### 16.2 Release 1

- Categoria 80/100.
- Inicio de mes.
- Orcamento nao revisado.
- Envio por template aprovado.
- Follow-up de categoria/orcamento.

### 16.3 Release 2

- Fechamento do mes.
- Retomada.
- Risco de abandono.
- Motivacao.

### 16.4 Release 3

- Meta atingida, somente apos prova/modelagem de meta individual.

## 17. Plano de implementacao

1. Acompanhar aprovacao dos templates Meta do escopo Release 1.
2. Criar contrato de template generico em `internal/platform/notification`.
3. Estender WhatsApp gateway para template generico usando client Meta existente.
4. Modelar tipos fechados de alerta e dedup.
5. Evoluir threshold 90 com migration segura.
6. Criar/evoluir persistencia de alertas proativos.
7. Implementar use case deterministico de avaliacao.
8. Implementar consumer de notificacao via outbox.
9. Persistir contexto minimo de follow-up.
10. Integrar follow-up ao agente usando ferramentas existentes.
11. Adicionar observabilidade.
12. Rodar em dry-run antes de habilitar envio.

## 18. Plano de testes

Unitarios:

- prioridade;
- dedup;
- threshold 80/100;
- supressao por frequencia;
- supressao por template ausente;
- renderizacao de parametros de template;
- follow-up com contexto valido/expirado.

Application:

- avaliar alertas sem duplicar;
- publicar outbox;
- nao marcar notificado quando Meta falha;
- resolver canal inexistente;
- respeitar assinatura ativa.

Repository:

- unique key;
- concorrencia de insert;
- listagem por pendente;
- update de estado.

Consumer:

- decode de evento;
- erro tipado;
- idempotencia de reprocessamento;
- dead-letter quando payload invalido.

Agentivo:

- resposta "sim" ao alerta de 80 chama detalhamento;
- resposta "sim" ao alerta de 90/100 chama panorama;
- resposta "sim" a orcamento ausente inicia `create_budget`;
- contexto expirado nao inventa intencao.

Meta:

- contract test com fake HTTP;
- teste de payload de template com componentes esperados;
- teste de rate limit e auth error.

## 19. Gates de validacao

Para implementacao futura:

```bash
gofmt -w <arquivos-go-alterados>
go build ./internal/budgets/... ./internal/agents/... ./internal/platform/notification/... ./internal/onboarding/...
go vet ./internal/budgets/... ./internal/agents/... ./internal/platform/notification/... ./internal/onboarding/...
go test -race -count=1 ./internal/budgets/... ./internal/agents/... ./internal/platform/notification/... ./internal/onboarding/...
```

Se houver migration:

- carregar `postgresql-production-standards`;
- rodar testes de migrations;
- validar rollback;
- validar constraints e indices.

Se tocar agente/plataforma:

- rodar gates Mastra;
- garantir kernel workflow sem dependencia de dominio/LLM;
- garantir tool/consumer fino.

## 20. Decisoes fechadas

- Release 1 envia no maximo um alerta iniciado pelo sistema por usuario por rodada diaria.
- Alertas elegiveis no Release 1: categoria 80%, categoria 100%, orcamento ausente e orcamento nao revisado.
- Threshold 90% nao entra no Release 1; exige VO, constraint, migration e `postgresql-production-standards`.
- Meta atingida nao entra sem entidade de meta individual provada no codebase.
- Dry-run e obrigatorio antes de envio real.
- Template Meta sem status `APPROVED` bloqueia envio real daquele kind.
- Texto livre nao e fallback permitido para proativo fora da janela WhatsApp.
- Templates `MARKETING` exigem opt-in explicito.
- Quiet hours: 20:00-08:00 no timezone do usuario, fallback `America/Sao_Paulo`.
- Persistencia generica `proactive_alerts` fica fora do Release 1; sera criada apenas quando o escopo sair de thresholds.

## 21. Gates objetivos

- Gate Meta: cada kind so pode sair do dry-run para envio real quando o template correspondente estiver `APPROVED`.
- Gate dry-run: envio real so pode ser habilitado apos pelo menos uma rodada operacional sem falsos positivos e sem volume inesperado.
- Gate Go: toda implementacao deve passar pelos gates de `go-implementation`.
- Gate Mastra: follow-up deve consumir `AgentRuntime`, Thread, Run e WorkingMemory existentes.
- Gate dominio: estados, kinds, supressoes e dedup devem ser tipos fechados e decisoes puras.
- Gate pattern: decisao atual e `nao aplicar padrao`; qualquer mudanca deve rerodar seletor.

## 22. Readiness

- Produto: fechado para Release 1.
- Dominio: fechado para Release 1.
- Arquitetura: fechada para PRD, TechSpec e tasks.
- Templates: criados/submetidos via Meta API; envio real condicionado por gate `APPROVED`.
- Implementacao: pronta para decomposicao e execucao por tasks.
- Production-ready: alcancavel por task, dry-run e gates objetivos; nao depende de suposicao nao decidida.
