<!-- spec-hash-prd: 3841dfedcc8147e79c764fa15a3c40d67438f9fea8a1f7afef343f45c970c2ad -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — Onboarding sem Fricção até Primeiro Lançamento Financeiro

## Resumo Executivo

A solução é uma evolução incremental do consumidor agentivo existente em `internal/agents`, preservando o runtime `internal/platform/{agent,memory,workflow,tool,scorer,llm}`, o workflow durável de onboarding e o fluxo `pending-entry` já responsável por confirmação humana e escrita financeira. Não haverá novo endpoint, novo bounded context, novo design pattern estrutural, migration PostgreSQL, feature flag, allowlist ou canary.

O fechamento técnico concentra-se em quatro frentes: ajustar a sequência do onboarding para que a primeira suspensão já contenha boas-vindas e objetivo sem consumir a resposta do usuário; endurecer a etapa de `💳` para aceitar banco/apelido único, recusar cartão sem bloquear onboarding e informar exatamente o dado faltante; corrigir o guard `card_provenance` para exigir `💳` apenas quando a tool realmente tenta `credit_card`; e transformar os critérios de primeiro lançamento em regressões unitárias, integração, golden/eval e pós-deploy observável.

Artefatos de entrada:
- PRD: `.specs/prd-onboarding-sem-friccao-ate-primeiro-lancamento/prd.md`.
- Discovery técnico: `discoveries/technical-onboarding-sem-friccao-ate-primeiro-lancamento-financeiro/`.
- Modelagem de domínio: `discoveries/domain-onboarding-sem-friccao-ate-primeiro-lancamento/`.
- ADRs: `adr-001-evolucao-incremental-internal-agents.md`, `adr-002-cartao-contextual-card-provenance.md`, `adr-003-rollout-sem-feature-flag.md`, `adr-004-slo-observabilidade-falso-sucesso.md`.

## Arquitetura do Sistema

### Visão Geral dos Componentes

Componentes modificados:
- `internal/agents/application/workflows/onboarding_workflow.go`: prompts, sequência efetiva `welcome -> goal`, decisão de `💳`, mensagens de conclusão e testes de persistência de mensagens do workflow.
- `internal/agents/application/agents/guards/card_provenance.go`: regra pós-tool para não substituir confirmações verbatim de pix, dinheiro, boleto, débito, TED, vale ou receita por pergunta indevida de `💳`.
- `internal/agents/application/agents/mecontrola_agent.go`: ordem de guards deve continuar preservando `verbatim_relay` antes de `card_provenance`; se o guard de cartão mudar, a ordem vira contrato testado.
- `internal/agents/application/workflows/pending_entry_decisions.go` e `pending_entry_workflow.go`: não devem mudar a regra central já correta; entram como regressão obrigatória para garantir que só `credit_card` abre `AwaitingSlotCard`.
- `internal/agents/application/tools/register_expense.go` e tool tests: validar que argumentos de `register_expense` com `paymentMethod != credit_card` não acionam exigência de `💳`, preservando confirmação verbatim.
- `internal/agents/application/golden/*`, `internal/agents/application/postdeploy/*` e testes E2E existentes: incluir casos do primeiro lançamento pix e receita simples com BRL em separador de milhar.
- `docs/alerts/*` ou runbook operacional equivalente: registrar alerta crítico de confirmação positiva sem transação ativa em até 30s.

Fluxo técnico alvo:
1. Ativação WhatsApp inicia o onboarding por `AgentRuntime.Execute` e workflow durável existente.
2. O passo `welcome` completa imediatamente quando não há `ResumeText`; ele existe para compatibilidade com definição e runs, mas não suspende com saudação isolada.
3. O passo `goal` suspende com a mensagem combinada exata do PRD quando `ResumeText == ""`; a próxima mensagem do usuário é processada pelo próprio `goal`, preservando o texto do objetivo.
4. Orçamento mensal usa a copy expandida com 5 categorias canônicas em linhas separadas.
5. A etapa de `💳` lista cartões ativos, pergunta por OUTRO `💳` quando houver cartão existente, aceita recusa como conclusão da etapa e cria cartão apenas após banco/apelido e vencimento válidos.
6. Lançamentos financeiros continuam pelo agente geral e pelas tools `register_expense`/`register_income`, que abrem `pending-entry`, pedem confirmação e só então persistem em `transactions`.
7. `card_provenance` inspeciona a sequência de tool calls e os argumentos da tool consumidora; somente `credit_card` sem resolução prévia de cartão pode forçar pergunta de `💳`.

## Design de Implementação

### Interfaces Chave

Nenhuma nova interface pública deve ser criada. A implementação consome os contratos existentes:

```go
type CardManager interface {
	CreateCard(ctx context.Context, in NewCard) (CardRef, error)
	ListCards(ctx context.Context, userID uuid.UUID) ([]Card, error)
}
```

```go
type TransactionsLedger interface {
	CreateTransaction(ctx context.Context, in RawTransaction) (EntryRef, error)
	UpdateTransaction(ctx context.Context, in RawUpdateTransaction) (EntryRef, error)
	CreateRecurringTemplate(ctx context.Context, in RawRecurringTemplate) (EntryRef, error)
}
```

Regras de interface:
- `CardManager` permanece interface consumida por `internal/agents/application`; nenhuma dependência de repository concreto entra no workflow.
- `TransactionsLedger` continua sendo a porta de escrita para `pending-entry`; não introduzir bypass de confirmação nem escrita direta.
- Tools permanecem adapters finos com schema estrito e `additionalProperties: false`.

### Modelos de Dados

Não há novo schema PostgreSQL.

Estados existentes preservados:
- `OnboardingState`: mantém `ResumeText`, `Phase`, `Goal`, `MonthlyBudgetCents`, `CardsDone` e campos atuais.
- `PendingEntryState`: mantém `PaymentMethod`, `CardID`, `OriginWamid`, `OriginOperation`, `AmountCents`, `Description`, categoria e confirmação.
- `workflow_runs`, `workflow_steps`, `platform_runs`, `platform_messages` e `outbox_events`: continuam como trilha auditável.

Mudanças de decisão em memória:
- Adicionar normalização pura para extração de `💳`: quando exatamente um dos campos `nickname` ou `bank` vier preenchido e `dueDay` for válido, usar o valor informado para ambos antes de chamar `CardManager.CreateCard`.
- Representar erro de cartão incompleto com detalhe verificável de campos faltantes. A implementação pode usar erro tipado ou retorno de decisão fechado; o handler do passo deve conseguir distinguir `nickname/bank` ausente de `dueDay` ausente para montar reprompt específico.
- Não persistir novos flags ou versões de onboarding.

### Endpoints de API

Não se aplica. A funcionalidade usa o canal WhatsApp inbound já existente e o consumidor em `internal/agents`. Não criar endpoint HTTP público, rota gRPC, webhook adicional ou política de autorização.

## Pontos de Integração

- WhatsApp inbound: preserva a prioridade de retomada de workflows antes do agente geral.
- OpenRouter/LLM: continua restrito às call-sites sancionadas do agente e dos passos do workflow; todo structured output segue schema estrito.
- PostgreSQL: sem migration; usa stores existentes para workflows, platform messages, outbox e `transactions`.
- Outbox: continua responsável por entrega assíncrona de resposta WhatsApp; idempotência por evento e origem permanece obrigatória.
- Observabilidade: métricas e traces existentes devem cobrir workflow, tool calls, guards, pending-entry, escrita financeira e outbox, sem labels com `user_id`, telefone, `wamid`, categoria ou IDs de entidade.

Falhas esperadas:
- LLM não extrai objetivo, orçamento ou cartão: workflow suspende com reprompt determinístico.
- `CardManager.CreateCard` falha: passo falha com erro contextual e não marca `CardsDone`.
- Confirmação positiva sem `resourceID`: `pending-entry` falha pelo contrato atual de `DecidePostWrite`.
- `TRANSACTIONS_ENABLED=false` ou `OUTBOX_DISPATCHER_ENABLED=false`: ambiente é considerado bloqueado para validação de produção.

## Abordagem de Testes

### Testes Unitários

Onboarding:
- `BuildWelcomeStep` completa sem suspender quando `ResumeText == ""`.
- `BuildGoalStep` suspende com a mensagem combinada exata do RF-03 e processa a próxima resposta como objetivo.
- `wrapStepWithMessages` registra uma única mensagem assistente para a primeira resposta combinada.
- `monthlyBudgetPrompt` contém a copy exata do RF-06, incluindo emojis e linhas das 5 categorias.
- `cardsPrompt(existing > 0)` usa `💳`, informa cartão existente e pergunta por `OUTRO 💳`.
- `cardsPrompt(existing == 0)` usa `💳` e deixa explícito que cadastro é opcional.
- Respostas `"Santander, vencimento dia 1"`, `"Nubank, vencimento dia 1"` e `"XP, vencimento dia 1"` criam `interfaces.NewCard` com `Nickname` e `Bank` preenchidos e `DueDay=1`.
- Resposta negativa sem cartão marca `CardsDone=true` sem chamar `CreateCard`.
- Resposta incompleta não chama `CreateCard`, não marca `CardsDone` e retorna reprompt específico do dado faltante usando `💳`.

Guards e agente:
- `card_provenance` não trata `register_expense` quando `paymentMethod` é `pix`, `cash`, `boleto`, `ted`, `debit_card`, `debit_in_account`, `vale_refeicao` ou `vale_alimentacao`.
- `card_provenance` trata `register_expense` com `paymentMethod=credit_card` e sem `resolve_card`/`list_cards` anterior.
- `verbatim_relay` continua podendo corrigir a resposta antes de `card_provenance`, e `card_provenance` não sobrescreve confirmação verbatim não relacionada a `credit_card`.
- Receita simples com `"Recebi R$ 13.874,40 de salário"` não ativa `multi_item`.

Pending-entry:
- `DecideInitialAwaiting` só retorna `AwaitingSlotCard` quando `paymentMethod == "credit_card"` e `hasCard == false`.
- Despesa pix com categoria resolvida chega a confirmação sem cartão.
- Após confirmação positiva, `buildRawTransaction` preserva `PaymentMethod`, `OriginWamid` e `OriginOperation`.

### Testes de Integração

Integration tests são obrigatórios porque o fluxo cruza WhatsApp inbound, workflow durável, stores PostgreSQL/outbox e escrita em `transactions`. Usar os harnesses já existentes do repositório; se o teste exigir banco real, manter build tag de integração conforme padrão local.

Cobertura mínima:
- Consumer WhatsApp retoma onboarding e envia primeira resposta combinada como uma única mensagem.
- Consumer WhatsApp retoma `pending-entry` antes do agente geral quando há pendência ativa.
- Despesa pix confirmada cria transação ativa com `amount_cents=5000`, descrição literal, `payment_method=pix`, categoria compatível e `origin_wamid` preenchido.
- Receita `"Recebi R$ 13.874,40 de salário"` cria ou confirma uma única receita com `amount_cents=1387440` e descrição literal `"salário"`.
- Fluxo de cartão em onboarding cria um único cartão ativo e não entra em loop.

### Testes E2E, Golden e Pós-Deploy

Golden/eval:
- Caso de onboarding inicial sem `"Oi"`.
- Caso de `💳` válido com banco/apelido único.
- Caso de recusa de `💳`.
- Caso de despesa pix sem pergunta de `💳`.
- Caso de receita com separador de milhar sem falso multi-lançamento.
- Scorer de `verbatim_required` deve continuar passando para confirmações retornadas por tools.

Pós-deploy:
- Executar jornada manual com o usuário de teste do requester, sem feature flag.
- Verificar `workflow_runs`, `workflow_steps`, `platform_messages`, `outbox_events`, `platform_runs` e linha ativa em `transactions`.
- Janela crítica: confirmação positiva deve gerar transação ativa em até 30s; ausência vira alerta crítico.

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. Atualizar prompts e sequência `welcome -> goal`: reduz fricção inicial e destrava os primeiros testes de mensagem única.
2. Atualizar copy e decisão de `💳`: fecha loops de cartão antes de mexer em lançamento financeiro.
3. Corrigir `card_provenance`: remove a pergunta indevida de `💳` para pix e preserva confirmações verbatim.
4. Reforçar regressões de `pending-entry`, pix e receita: prova que a escrita financeira chega a `transactions` com confirmação.
5. Atualizar golden/eval e pós-deploy: fecha risco de falso positivo conversacional.
6. Atualizar alertas/runbook e checklist de rollout sem feature flag.

### Dependências Técnicas

Bloqueantes para validação production-ready:
- `TRANSACTIONS_ENABLED=true`.
- `OUTBOX_DISPATCHER_ENABLED=true`.
- OpenRouter configurado e funcional.
- Timeout efetivo do inbound/outbox compatível com a jornada de confirmação; qualquer mismatch que interrompa handler antes de concluir deve ser corrigido ou documentado como bloqueio de deploy.
- Ambiente de integração com PostgreSQL e migrations atuais.

Não bloqueantes:
- Nenhuma migration nova.
- Nenhum provedor externo novo.
- Nenhuma dependência Go nova prevista.

## Monitoramento e Observabilidade

Métricas e sinais obrigatórios:
- Taxa de ativações com primeira mensagem combinada emitida.
- Tempo de ativação até primeira transação ativa, excluindo espera do usuário; SLO: 95% em até 5 minutos.
- Webhook inbound p95 < 500ms nos testes de carga existentes.
- Contagem de decisões do guard `card_provenance` por `agent_id`, `guard` e `decision`, sem labels de usuário.
- Confirmações positivas sem `resourceID` ou sem transação ativa em até 30s.
- Falhas de outbox, dead-letter e retries por status controlado.
- Regressão de custo por jornada; bloquear rollout se staging exceder baseline em mais de 30%.

Logs/traces:
- Registrar erro contextual em falhas de parse, unmarshal, create card, list cards, write ledger e outbox.
- Não registrar telefone, `wamid`, `user_id`, texto livre sensível ou descrição financeira como label de métrica.
- Traces devem permitir correlacionar inbound, workflow resume, tool call, pending-entry e escrita financeira por IDs internos já existentes.

Alertas:
- Crítico: confirmação positiva sem transação ativa em até 30s.
- Crítico: dead-letter de WhatsApp ou outbox durante jornada de onboarding/primeiro lançamento.
- Warning: aumento de `card_provenance` handled em `paymentMethod != credit_card` deve ser zero; se ocorrer, é regressão.

## Considerações Técnicas

### Decisões Chave

- Evolução incremental em `internal/agents`, sem novo workflow ou bounded context. ADR: `adr-001-evolucao-incremental-internal-agents.md`.
- `💳` é opcional no onboarding e contextual no lançamento; `card_provenance` só exige cartão em `credit_card`. ADR: `adr-002-cartao-contextual-card-provenance.md`.
- Rollout sem feature flag, allowlist ou canary por decisão explícita do requester; validação manual usa apenas usuário de teste, mas o deploy afeta produção geral. ADR: `adr-003-rollout-sem-feature-flag.md`.
- SLO e alertas focam falso sucesso financeiro, não só resposta conversacional. ADR: `adr-004-slo-observabilidade-falso-sucesso.md`.

### Riscos Conhecidos

- Deploy sem feature flag expõe todos os usuários ao novo fluxo no momento do release. Mitigação: testes completos antes do deploy, checklist de configs, jornada manual imediata e rollback por reversão de deploy.
- `welcome` já pode ter runs suspensos antigos. Mitigação: preservar `stepWelcomeID` na definição; alteração nova deve ser compatível com runs que retomam pela sequência existente. Se houver run legado suspenso em `welcome`, validar em teste de resume antes do deploy.
- Guard pós-tool pode sobrescrever texto correto se a inspeção de argumentos for incompleta. Mitigação: parser de argumentos estrito e tabela de testes por payment method.
- Structured output de cartão pode preencher só `bank` ou só `nickname`. Mitigação: normalização determinística antes de `DecideCardEntry` e testes com bancos do PRD.
- Receita com separador de milhar pode ser confundida com múltiplos valores. Mitigação: teste unitário/golden em multi-item e e2e com `"Recebi R$ 13.874,40 de salário"`.

### Conformidade com Padrões

- `AGENTS.md`: fluxo permitido `infrastructure -> application -> domain`; sem violação de camadas.
- `go-implementation`: validar gofmt, build, vet, race tests e lint proporcional ao escopo alterado.
- `mastra`: usar `internal/platform/{agent,llm,memory,workflow,tool,scorer}` e consumidor real `internal/agents`; não reimplementar Thread, Run, WorkingMemory ou PendingStep.
- `domain-modeling-production`: manter state-as-type, smart constructor quando aplicável, decisões puras e workflow `parse -> validate -> decide -> persist -> publish`.
- `design-patterns-mandatory`: decisão é `nao aplicar padrao`; a correção é localizada e não justifica nova abstração estrutural.
- Segurança: sem novo canal, sem nova autorização, sem labels de PII e sem dados financeiros sensíveis em métricas.
- PostgreSQL: sem migration; se a implementação descobrir necessidade de schema, esta techspec deve ser revisada antes de código.

### Arquivos Relevantes e Dependentes

- `internal/agents/application/workflows/onboarding_workflow.go`
- `internal/agents/application/workflows/onboarding_workflow_test.go`
- `internal/agents/application/workflows/onboarding_workflow_integration_test.go`
- `internal/agents/application/workflows/pending_entry_decisions.go`
- `internal/agents/application/workflows/pending_entry_workflow.go`
- `internal/agents/application/workflows/pending_entry_workflow_test.go`
- `internal/agents/application/agents/guards/card_provenance.go`
- `internal/agents/application/agents/guards/card_provenance_test.go`
- `internal/agents/application/agents/guard_chain.go`
- `internal/agents/application/agents/mecontrola_agent.go`
- `internal/agents/application/agents/mecontrola_agent_gherkin_e2e_test.go`
- `internal/agents/application/tools/register_expense.go`
- `internal/agents/application/tools/financial_tools_test.go`
- `internal/agents/application/tools/register_expense_integration_test.go`
- `internal/agents/application/golden/*`
- `internal/agents/application/postdeploy/*`
- `internal/transactions/application/usecases/create_transaction.go`
- `internal/transactions/infrastructure/repositories/postgres/transaction_repository.go`
- `docs/alerts/whatsapp-dead-letter.yaml`
- `docs/alerts/mecontrola-agent-gate-posdeploy.yaml`
- `scripts/loadtest/whatsapp-inbound.js`
- `taskfiles/loadtest.yml`

## Mapeamento de Requisitos para Decisão e Teste

| Requisitos | Decisão técnica | Teste mínimo |
| --- | --- | --- |
| RF-01 a RF-03 | `welcome` não suspende; `goal` suspende com copy combinada | Unit + integração de primeira mensagem única |
| RF-04 a RF-06 | `monthlyBudgetPrompt` substituído pela copy funcional | Unit de string exata |
| RF-07 a RF-15 | `💳` em toda copy; normalização banco/apelido; reprompt específico | Unit de prompt, criação, recusa e incompleto |
| RF-16 a RF-19 | `card_provenance` condicionado a `credit_card`; pending-entry preservado | Unit de guard + integração pix confirmada |
| RF-20 a RF-23 | Multi-item não dispara para BRL único; receita preserva descrição literal | Unit/golden + e2e receita |
| RF-24 a RF-27 | Sucesso só após retorno real de tool/use case; confirmação antes de write | Tool tests + pending-entry integration |
| RF-28 a RF-30 | Ordem de retomada WhatsApp preservada; sem endpoint novo | Integração do consumer |
| RF-31 a RF-34 | Auditoria e métricas sem PII/high cardinality | Teste/inspeção de métricas e runbook |
| RF-35 a RF-39 | Gates completos por camada | `go test -race`, integração, golden/eval e pós-deploy |

## Validação Obrigatória

Comandos mínimos antes de considerar implementação pronta:

```bash
rtk gofmt -w <arquivos-go-alterados>
rtk go test -race -count=1 ./internal/agents/application/workflows/...
rtk go test -race -count=1 ./internal/agents/application/agents/...
rtk go test -race -count=1 ./internal/agents/application/tools/...
rtk go test -race -count=1 ./internal/agents/...
rtk go vet ./internal/agents/...
rtk golangci-lint run ./internal/agents/...
```

Gates complementares quando disponíveis no repositório:

```bash
rtk task agents:integration
rtk task test:golden:gate
rtk task ci:pipeline
rtk task loadtest:whatsapp
```

Critério de fechamento:
- Todos os testes e gates acima passam.
- Nenhum falso positivo conversacional é aceito.
- Jornada manual pós-deploy do requester cria transação ativa rastreável.
- Nenhuma pergunta de `💳` aparece para pix, dinheiro, boleto, débito, TED, vale ou receita.

## Itens em Aberto

Nenhum item técnico bloqueante permanece em aberto para gerar tarefas. Risco residual explicitamente aceito: deploy sem feature flag/allowlist/canary, com validação manual pelo usuário de teste do requester e rollback por reversão de deploy.
