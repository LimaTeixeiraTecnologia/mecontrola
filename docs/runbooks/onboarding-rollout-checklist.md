# Runbook — Rollout e Validação Pós-Deploy: Onboarding sem Fricção até Primeiro Lançamento Financeiro

- **PRD:** `.specs/prd-onboarding-sem-friccao-ate-primeiro-lancamento/prd.md`
- **TechSpec:** `.specs/prd-onboarding-sem-friccao-ate-primeiro-lancamento/techspec.md`
- **ADRs:** `.specs/prd-onboarding-sem-friccao-ate-primeiro-lancamento/adr-003-rollout-sem-feature-flag.md`, `.specs/prd-onboarding-sem-friccao-ate-primeiro-lancamento/adr-004-slo-observabilidade-falso-sucesso.md`
- **Runbook relacionado:** `docs/runbooks/mecontrola-agent-gate-posdeploy.md`
- **Alertas relacionados:** `docs/alerts/mecontrola-agent-gate-posdeploy.yaml`, `docs/alerts/whatsapp-dead-letter.yaml`
- **Dashboard relacionado:** `docs/dashboards/mecontrola-agent-gate-posdeploy.json`

## 1. Escopo e decisões de rollout

Este runbook aplica-se ao deploy da funcionalidade **Onboarding sem Fricção até Primeiro Lançamento Financeiro**.

Decisões arquiteturais fixas:

- **Sem feature flag, allowlist ou canary.** O release expõe todos os usuários ao novo comportamento no momento do deploy. Não há kill switch específico para esta funcionalidade.
- **Rollback é reversão de deploy.** Não há desligamento seletivo.
- **Critério de sucesso operacional:** ativação até primeira transação ativa rastreável, não apenas mensagem enviada ou workflow concluído.

## 2. Pré-requisitos antes do deploy

Antes de iniciar o rollout, todos os itens abaixo devem estar verificados.

### 2.1 Gates de qualidade fechados

- [ ] Tarefas 1.0 a 8.0 do PRD concluídas e com relatórios de execução em `.specs/prd-onboarding-sem-friccao-ate-primeiro-lancamento/`.
- [ ] Testes unitários/integração da funcionalidade passam:
  - `go test -race -count=1 ./internal/agents/application/workflows/...`
  - `go test -race -count=1 ./internal/agents/application/agents/...`
  - `go test -race -count=1 ./internal/agents/application/tools/...`
  - `go test -race -count=1 ./internal/agents/...`
- [ ] Golden/eval passam: `task test:golden:gate` (ou comando equivalente do repositório).
- [ ] Contrato de regressão passa: `go test ./internal/agents/application/postdeploy/... -run TestRegressionContractSuite -v`.
- [ ] Lint limpo no escopo alterado: `golangci-lint run ./internal/agents/...`.
- [ ] Build limpo: `go build ./...`.

### 2.2 Configurações obrigatórias de produção

Verificar no arquivo de configuração de produção (`deployment/config/prod.env`) e/ou nas variáveis de ambiente efetivas no servidor:

| Variável | Valor esperado | Onde verificar | Bloqueante? |
|---|---|---|---|
| `TRANSACTIONS_ENABLED` | `true` | `deployment/config/prod.env` e env do worker/api | Sim. Se `false`, confirmações positivas não persistem transações ativas. |
| `OUTBOX_DISPATCHER_ENABLED` | `true` | `deployment/config/prod.env` e env do worker | Sim. Se `false`, respostas WhatsApp não saem. |
| `OPENROUTER_BASE_URL` | `https://openrouter.ai` (ou endpoint funcional) | `deployment/config/prod.env` e secrets | Sim. LLM deve estar acessível para extração de objetivo, orçamento e cartão. |
| `AGENT_LLM_PRIMARY_MODEL` | Modelo configurado e funcional (ex: `openai/gpt-4o-mini`) | `deployment/config/prod.env` | Sim. |
| `META_BOT_NUMBER_E164` | Número do bot configurado | `deployment/config/prod.env` | Sim. |
| `OUTBOX_DISPATCHER_HANDLER_TIMEOUT` | Compatível com jornada de confirmação (padrão `10s`) | `deployment/config/prod.env` | Sim, se menor que o tempo necessário para processar confirmação + write. |
| `WHATSAPP_WEBHOOK_RATE_LIMIT_*` | Configurado para carga esperada | `deployment/config/prod.env` | Não, mas revisar. |

Comando de verificação rápida no container/servidor de produção:

```bash
# Verificar valores efetivos no ambiente do serviço
systemctl show-environment | grep -E 'TRANSACTIONS_ENABLED|OUTBOX_DISPATCHER_ENABLED|OPENROUTER'
# ou, se estiver usando docker compose:
docker compose -f deployment/compose/<arquivo>.yml exec api printenv | grep -E 'TRANSACTIONS_ENABLED|OUTBOX_DISPATCHER_ENABLED|OPENROUTER'
docker compose -f deployment/compose/<arquivo>.yml exec worker printenv | grep -E 'TRANSACTIONS_ENABLED|OUTBOX_DISPATCHER_ENABLED|OPENROUTER'
```

### 2.3 Observabilidade pronta

- [ ] Dashboard `docs/dashboards/mecontrola-agent-gate-posdeploy.json` importado/atualizado no Grafana.
- [ ] Alertas `docs/alerts/mecontrola-agent-gate-posdeploy.yaml` e `docs/alerts/whatsapp-dead-letter.yaml` aplicados no Alertmanager/Prometheus.
- [ ] Canal de notificação de alertas críticos configurado e testado.
- [ ] Acesso a Prometheus, Tempo e Postgres de produção confirmado para o operador.

## 3. Procedimento de deploy

1. [ ] Anotar a versão/imagem do deploy anterior (baseline) e o timestamp de início.
2. [ ] Realizar o deploy pela pipeline ou procedimento oficial do serviço (`task ci:deploy`, GitHub Actions, ou manual conforme runbook de infra).
3. [ ] Confirmar que os containers/serviços `api` e `worker` estão saudáveis após o deploy.
4. [ ] Anotar o timestamp do deploy no dashboard (anotação "Deploy onboarding sem fricção") e neste runbook.
5. [ ] Verificar nos logs que não houve panic ou erro de inicialização nos primeiros 2 minutos.

## 4. Validação pós-deploy — jornada manual com usuário de teste do requester

> Esta jornada deve ser executada pelo requester ou por operador designado com acesso ao usuário de teste do WhatsApp. O agente de IA documenta o procedimento; a execução manual é responsabilidade humana.

### 4.1 Dados do teste

Preencher antes de iniciar:

- **Data/hora do início:**
- **Telefone/número do usuário de teste:**
- **User ID no MeControla (se conhecido):**
- **Versão/imagem do deploy:**
- **Operador:**

### 4.2 Passo a passo da jornada

| # | Ação do usuário de teste | Resposta esperada do sistema | Check |
|---|---|---|---|
| 1 | Enviar "Ativar o meu plano" (ou concluir ativação pelo fluxo de checkout) | Recebe uma **única** mensagem com `🎉 Bem-vindo ao MeControla! 🎉` e `Vamos começar? Qual é o seu principal objetivo financeiro para este mês?` | [ ] |
| 2 | **NÃO enviar "Oi".** Responder objetivo, ex: `Quero economizar R$ 500` | Sistema aceita objetivo e avança para orçamento | [ ] |
| 3 | Receber mensagem de orçamento | Mensagem começa com `📊 Antes de montar seu planejamento`, lista as 5 categorias em linhas separadas e pergunta `Qual é o seu orçamento mensal?` | [ ] |
| 4 | Responder orçamento, ex: `R$ 3.500,00` | Sistema aceita orçamento e avança | [ ] |
| 5 | Na etapa de `💳`, responder `Santander, vencimento dia 1` | Sistema cria um `💳` ativo e pergunta `Deseja cadastrar OUTRO 💳?` | [ ] |
| 6 | Responder `não` à pergunta de OUTRO `💳` | Onboarding conclui sem loop de cartão | [ ] |
| 7 | Enviar `gastei R$ 50,00 no supermercado no pix` | Sistema **não pergunta qual `💳`**; solicita data | [ ] |
| 8 | Responder `hoje` | Sistema exibe confirmação contendo `supermercado`, `R$ 50,00`, categoria e `pix` | [ ] |
| 9 | Responder `sim` à confirmação | Sistema confirma e **persiste** transação ativa | [ ] |
| 10 | Enviar `Recebi R$ 13.874,40 de salário` | Sistema **não** responde que percebeu mais de um lançamento; inicia confirmação mínima de receita única | [ ] |
| 11 | Confirmar a receita com `sim` | Sistema persiste transação ativa de receita com valor `R$ 13.874,40` e descrição literal `salário` | [ ] |

### 4.3 Critérios de falha imediata (rollback sem aguardar janela)

Se qualquer um dos itens abaixo ocorrer durante a jornada manual, **reverter o deploy imediatamente**:

- O usuário precisa enviar "Oi" para receber a pergunta de objetivo.
- A mensagem de orçamento não exibe as 5 categorias com emoji e descrição curta.
- Resposta válida de cartão (`Santander, vencimento dia 1`) entra em loop.
- Resposta `não` na etapa de `💳` (sem cartão) não conclui o onboarding.
- Pergunta de `💳` aparece para pagamento `pix`, `dinheiro`, `boleto`, `débito`, `TED`, `vale_refeicao`, `vale_alimentacao` ou receita.
- Confirmação positiva não gera transação ativa rastreável em até 30 segundos.
- Resposta de múltiplos lançamentos para valor BRL único com separador de milhar.

## 5. Verificação em banco de dados

Após a jornada manual, executar as consultas abaixo para confirmar rastreabilidade fim a fim. Nunca expor `user_id`, telefone, `wamid` ou IDs de entidade em labels de métrica; use-os apenas como parâmetros de consulta ad hoc.

### 5.1 Onboarding

```sql
SELECT id, workflow, status, phase, cards_done, goal, goal_value_cents, monthly_budget_cents, recurrence, started_at, ended_at
FROM mecontrola.workflow_runs
WHERE workflow = 'onboarding-workflow'
  AND user_id = '<user-id-do-teste>'
ORDER BY started_at DESC
LIMIT 5;
```

Critérios:

- `status = 'succeeded'`.
- `phase` deve refletir conclusão.
- `cards_done = true` somente se houver cartão ativo ou recusa explícita.
- `goal` e `monthly_budget_cents` preenchidos.

### 5.2 Workflow steps de onboarding

```sql
SELECT id, run_id, step, status, output, started_at, ended_at
FROM mecontrola.workflow_steps
WHERE run_id IN (
  SELECT id
  FROM mecontrola.workflow_runs
  WHERE workflow = 'onboarding-workflow'
    AND user_id = '<user-id-do-teste>'
)
ORDER BY started_at DESC;
```

### 5.3 Mensagens do WhatsApp

```sql
SELECT id, run_id, direction, status, sent_at, delivered_at
FROM mecontrola.platform_messages
WHERE run_id IN (
  SELECT id
  FROM mecontrola.workflow_runs
  WHERE workflow = 'onboarding-workflow'
    AND user_id = '<user-id-do-teste>'
)
ORDER BY created_at DESC
LIMIT 50;
```

Confirmar:

- Primeira mensagem outbound contém `Bem-vindo ao MeControla` e `Qual é o seu principal objetivo financeiro`.
- Mensagem de orçamento contém as 5 categorias.
- Não há mensagens de erro ou loop de `💳`.

### 5.4 Transações ativas

```sql
SELECT id, direction, amount_cents, description, payment_method, category_path, origin_wamid, origin_operation, created_at
FROM mecontrola.transactions
WHERE user_id = '<user-id-do-teste>'
  AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT 10;
```

Critérios:

- Despesa pix: `direction = 'expense'`, `amount_cents = 5000`, `description = 'supermercado'`, `payment_method = 'pix'`, `category_path` compatível com `Custo Fixo > Supermercado`, `origin_wamid` preenchido.
- Receita salário: `direction = 'income'`, `amount_cents = 1387440`, `description = 'salário'`, `origin_wamid` preenchido.

### 5.5 Outbox

```sql
SELECT id, event_type, status, attempts, dispatched_at, error
FROM mecontrola.outbox_events
WHERE origin_user_id = '<user-id-do-teste>'
ORDER BY created_at DESC
LIMIT 50;
```

Confirmar:

- Todas as mensagens outbound da jornada estão `status = 'dispatched'`.
- Nenhum evento com `status = 'failed'` ou `attempts` no limite.

### 5.6 Ledger de escrita idempotente

```sql
SELECT id, operation, resource_id, resource_kind, wamid, created_at
FROM mecontrola.agents_write_ledger
WHERE wamid IN (
  SELECT origin_wamid
  FROM mecontrola.transactions
  WHERE user_id = '<user-id-do-teste>'
    AND deleted_at IS NULL
)
ORDER BY created_at DESC;
```

Confirmar:

- `resource_id` preenchido para cada confirmação positiva.
- Nenhuma linha de confirmação positiva com `resource_id` nulo.

### 5.7 Platform runs do agente

```sql
SELECT id, agent_id, status, outcome, started_at, ended_at
FROM mecontrola.platform_runs
WHERE user_id = '<user-id-do-teste>'
ORDER BY started_at DESC
LIMIT 20;
```

## 6. SLO e observabilidade

### 6.1 Métricas a monitorar

| Métrica/SLO | Alvo | Consulta/exemplo |
|---|---|---|
| Ativação até primeira transação ativa | 95% em ≤ 5 minutos, excluindo espera do usuário | Ver seção 6.2 |
| Confirmação positiva sem recurso durável | 0 em ≤ 30s | `sum(increase({__name__=~"agents_.+_false_success_total"}[5m]))` |
| Dead-letter/outbox | 0 durante jornada | `increase(whatsapp_dead_letter_total[5m])` e outbox failed |
| Webhook inbound p95 | < 500ms | `histogram_quantile(0.95, rate(whatsapp_webhook_duration_seconds_bucket[5m]))` |
| Regressão de custo por jornada | ≤ baseline + 30% | Comparar com baseline de staging |

### 6.2 Cálculo do SLO "ativação até primeira transação ativa"

Este SLO é calculado manualmente na janela pós-deploy. Não expor `user_id` como label.

Passo 1: identificar ativações no período.

```sql
SELECT count(*) AS activations
FROM mecontrola.workflow_runs
WHERE workflow = 'onboarding-workflow'
  AND status = 'succeeded'
  AND ended_at >= '<timestamp-do-deploy>';
```

Passo 2: identificar ativações que geraram transação ativa em até 5 minutos após conclusão do onboarding.

```sql
WITH onboarded AS (
  SELECT user_id, ended_at AS onboarding_ended_at
  FROM mecontrola.workflow_runs
  WHERE workflow = 'onboarding-workflow'
    AND status = 'succeeded'
    AND ended_at >= '<timestamp-do-deploy>'
)
SELECT count(DISTINCT o.user_id) AS activated_with_transaction
FROM onboarded o
JOIN mecontrola.transactions t
  ON t.user_id = o.user_id
  AND t.deleted_at IS NULL
  AND t.created_at >= o.onboarding_ended_at
  AND t.created_at <= o.onboarding_ended_at + interval '5 minutes';
```

> **Nota sobre espera do usuário:** o SLO exclui tempo de espera por resposta do usuário. Na prática, isso significa medir o intervalo entre o fim do onboarding e a primeira transação ativa, desde que a mensagem do usuário tenha sido recebida dentro da janela. Documentar exceções manualmente.

Passo 3: computar taxa.

```text
taxa = activated_with_transaction / activations
```

Se `taxa < 0.95`, investigar causas (falta de cartão, confirmação não persistida, outbox atrasado, etc.) antes de promover a versão.

## 7. Rollback por reversão de deploy

> **Sem feature flag:** a única forma de desfazer o rollout é reverter o deploy para a versão anterior.

### 7.1 Gatilhos de rollback imediato

Qualquer um dos seguintes eventos dispara rollback **sem aguardar janela de observação**:

- Falso sucesso financeiro confirmado: confirmação positiva sem transação ativa em até 30s.
- Loop de `💳` para resposta válida (`Santander, vencimento dia 1`, `Nubank, vencimento dia 1`, `XP, vencimento dia 1`).
- Pergunta de `💳` para pagamento `pix`, `dinheiro`, `boleto`, `débito`, `TED`, `vale_refeicao`, `vale_alimentacao` ou receita.
- Primeira mensagem não combina boas-vindas + objetivo (exige "Oi" do usuário).
- Falso múltiplo lançamento para valor BRL único com separador de milhar.
- Dead-letter ou outbox failed durante jornada de onboarding/primeiro lançamento.
- Qualquer panic ou erro de inicialização massivo nos logs após deploy.

### 7.2 Procedimento de rollback

1. [ ] Identificar a versão anterior (baseline) anotada na seção 3.
2. [ ] Executar a reversão pelo procedimento oficial:
   - Se via GitHub Actions: reexecutar o workflow de deploy apontando para a tag/branch anterior.
   - Se via Docker Compose: `docker compose -f deployment/compose/<arquivo>.yml pull <imagem-anterior>` e recriar containers.
   - Se via VPS/systemd: restaurar a versão anterior do binário e restartar serviços.
3. [ ] Confirmar que `api` e `worker` estão saudáveis na versão anterior.
4. [ ] Anotar timestamp do rollback no dashboard e neste runbook.
5. [ ] Notificar requester e abrir incidente se houver falso sucesso financeiro ou regressão crítica.
6. [ ] Após estabilização, revisar logs, métricas e traces para identificar causa raiz antes de novo deploy.

### 7.3 Rollback parcial (quando aplicável)

Como não há feature flag, **não é possível** desligar apenas a funcionalidade de onboarding. Se a reversão total for inviável por dependência de outras mudanças no mesmo deploy, tratar como incidente e aplicar hotfix prioritário.

## 8. Checklist final de fechamento

Após jornada manual e verificações:

- [ ] Primeira mensagem outbound contém boas-vindas + objetivo numa única mensagem.
- [ ] Mensagem de orçamento exibe as 5 categorias com emoji e descrição curta.
- [ ] Resposta válida de `💳` cria cartão sem loop.
- [ ] Recusa de `💳` permite concluir onboarding sem erro.
- [ ] Despesa pix chega à confirmação sem perguntar `💳`.
- [ ] Confirmação positiva de despesa pix gera transação ativa rastreável.
- [ ] Receita simples com separador de milhar não vira múltiplo lançamento.
- [ ] Confirmação positiva de receita gera transação ativa rastreável.
- [ ] Nenhum alerta crítico de falso sucesso financeiro disparado.
- [ ] Nenhum evento de outbox ou WhatsApp em dead-letter durante a jornada.
- [ ] SLO "ativação até primeira transação ativa" atende ≥ 95% na amostra observada.
- [ ] Rollback documentado e versão anterior identificada.
- [ ] Decisão de manter/promover/reverter registrada com timestamp, evidências e operador.

## 9. Referências

- PRD: `.specs/prd-onboarding-sem-friccao-ate-primeiro-lancamento/prd.md`
- TechSpec: `.specs/prd-onboarding-sem-friccao-ate-primeiro-lancamento/techspec.md`
- ADR-003: `.specs/prd-onboarding-sem-friccao-ate-primeiro-lancamento/adr-003-rollout-sem-feature-flag.md`
- ADR-004: `.specs/prd-onboarding-sem-friccao-ate-primeiro-lancamento/adr-004-slo-observabilidade-falso-sucesso.md`
- Runbook de gate pós-deploy: `docs/runbooks/mecontrola-agent-gate-posdeploy.md`
- Alertas: `docs/alerts/mecontrola-agent-gate-posdeploy.yaml`, `docs/alerts/whatsapp-dead-letter.yaml`
- Dashboard: `docs/dashboards/mecontrola-agent-gate-posdeploy.json`
