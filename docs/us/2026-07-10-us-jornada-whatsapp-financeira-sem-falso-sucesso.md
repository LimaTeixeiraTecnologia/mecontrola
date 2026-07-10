# US-001: Jornada WhatsApp financeira sem falso sucesso

## Declaração
Como cliente ativa do MeControla no WhatsApp, quero concluir onboarding, personalizar meu orçamento e registrar despesas por conversa com confirmação, persistência idempotente e rastreabilidade ponta a ponta, para confiar que cada resposta de sucesso corresponde exatamente ao estado financeiro gravado na minha conta.

## Contexto
- Problema: na janela produtiva dos últimos 7 dias, a jornada do usuário `3140d64a-6010-4464-8bb6-cf8be1b10035` ativou assinatura, concluiu onboarding, criou cartão e orçamento, mas apresentou quebras críticas: personalização de orçamento enviada pelo usuário não foi persistida, despesa simples foi tratada como orientação de múltiplos lançamentos, confirmação de despesa gerou nova confirmação em vez de sucesso, segunda confirmação terminou sem transação nem ledger, e runs do agente ficaram como `succeeded/routed` sem `correlation_key` por WAMID.
- Resultado esperado: cada inbound WhatsApp financeiro deve caminhar por `dedup -> identidade -> entitlement -> resumer/workflow -> tool/use case -> ledger/transação/orçamento -> mensagem -> run e scorer e trace`, sem sucesso aparente quando o efeito financeiro não foi gravado, sem falso positivo de tool-call accuracy, sem perda de mensagem da conversa e sem quebra de personalização.
- Fonte: análise do codebase atual, SSH em `root@187.77.45.48`, Postgres de produção, outbox, runs, workflow snapshots, scorers, logs Docker, métricas Prometheus/OTel e busca Tempo na janela de `2026-07-03 11:36 UTC` a `2026-07-10 11:36 UTC`.

## Regras de Negócio
- A jornada deve preservar a linguagem de domínio existente: ativação, onboarding, cartão, orçamento, alocação, lançamento, despesa, confirmação, pendência, ledger, transação, replay e cancelamento.
- Uma resposta de sucesso financeiro só pode ser enviada quando o estado durável correspondente existir no banco: orçamento ativado com alocações confirmadas, ledger escrito, transação criada, mensagem persistida e outbox publicado quando houver efeito assíncrono.
- Personalização de orçamento enviada pelo usuário deve substituir a sugestão padrão somente quando a soma das alocações fechar 100%; se não fechar, o workflow deve pedir correção antes de ativar.
- Uma mensagem com uma única despesa, um único valor, uma descrição, uma forma de pagamento e data inferível deve iniciar uma pendência de registro ou registrar após confirmação; não pode cair no texto de múltiplos lançamentos.
- Confirmação afirmativa repetida para a mesma pendência deve ser tratada como replay idempotente quando o primeiro efeito já foi gravado, ou como retry controlado quando o efeito anterior falhou; nunca deve gerar uma segunda confirmação indistinguível nem cancelar com erro genérico sem causa auditável.
- O workflow `pending-entry` não pode concluir com `StepStatusCompleted` e `PendingStatusCancelled` quando a intenção aceita era registrar e a escrita retornou `resourceID` vazio; esse estado deve ser erro tipado, retry controlado ou resposta de falha auditável.
- `platform_runs.status`, `workflow_runs.status`, `workflow_runs.state.status`, `agents_write_ledger`, `transactions` e `platform_scorer_results` devem concordar sobre o resultado final: sucesso persistido, replay persistido, cancelamento explícito do usuário, expiração ou falha.
- Scorers não podem considerar uma jornada de escrita como correta apenas por tool invocada; devem verificar efeito final em ledger/transação/orçamento e mensagem de sucesso compatível.
- Observabilidade da jornada deve permitir reconstrução por `user_id`, `thread_id`, WAMID, `run_id`, `workflow_run_id`, `outbox_event_id` e status persistidos, sem adicionar labels de métrica com alta cardinalidade como `user_id` ou conteúdo de mensagem.
- Status de entrega WhatsApp deve ser registrado quando a Cloud API enviar status webhook; se o provider não enviar status, a ausência deve ser distinguível de falha de persistência.
- A resolução de identidade deve funcionar pelo caminho canônico de `user_identities`; quando o legado `users.whatsapp_number` for usado, o sistema deve registrar trilha auditável e permitir backfill seguro da identidade ativa.
- A implementação deve seguir `$go-implementation`, `$mastra`, `$domain-modeling-production` e `$design-patterns-mandatory`: usar os primitives atuais de Thread, Run, WorkingMemory, PendingStep, workflow e tool adapter fino; a decisão de pattern é não introduzir novo GoF pattern, porque o seletor indicou solução direta/refactor local sobre workflows e adapters existentes.

## Critérios de Aceite
```gherkin
Cenário: onboarding personalizado persiste exatamente a distribuição confirmada
  Dado uma cliente ativa com assinatura vigente e WhatsApp autenticado
  E o onboarding coletou objetivo, renda mensal, cartão e uma distribuição personalizada que soma 100%
  Quando a cliente revisar e ativar o orçamento
  Então o orçamento deve ser criado e ativado com os mesmos valores confirmados pela cliente
  E nenhuma alocação padrão deve sobrescrever a personalização enviada
  E `workflow_runs`, `budgets`, `budgets_allocations`, `platform_messages` e outbox devem permitir reconstruir a decisão completa

Cenário: personalização inválida de orçamento não vira orçamento ativo
  Dado uma distribuição personalizada que não soma 100%
  Quando a cliente enviar a distribuição durante o onboarding ou workflow de orçamento
  Então o sistema deve responder pedindo ajuste objetivo dos percentuais ou valores
  E o orçamento deve permanecer pendente, sem ativação parcial
  E o workflow deve registrar estado de espera tipado para correção de alocação

Cenário: despesa simples não é classificada como múltiplos lançamentos
  Dado a mensagem "Gastei 10 na padaria no dinheiro"
  Quando o inbound WhatsApp passar pelo agente financeiro
  Então o sistema deve identificar uma única despesa com valor 1000, descrição "padaria" e pagamento em dinheiro
  E deve pedir apenas os dados obrigatórios ausentes ou a confirmação final
  E não deve responder a orientação de múltiplos lançamentos

Cenário: despesa com valor, descrição, pagamento e data confirma uma única vez e grava
  Dado a mensagem "Gastei 19 na padaria no Pix"
  E a cliente responde "Hoje"
  Quando o sistema pedir confirmação e a cliente responder "Sim"
  Então deve existir exatamente um registro em `agents_write_ledger`
  E deve existir exatamente uma transação vinculada ao WAMID original, usuário e operação de registro
  E a resposta final deve informar sucesso do registro, não uma nova pergunta de confirmação
  E `platform_messages` deve conter a mensagem inbound e a resposta final da pendência

Cenário: confirmação duplicada vira replay idempotente ou retry controlado
  Dado uma pendência de despesa aguardando confirmação
  Quando a cliente enviar "Sim" duas vezes com WAMIDs diferentes
  Então a primeira confirmação deve criar a transação ou registrar falha tipada
  E a segunda confirmação deve responder como replay idempotente se a transação já existir
  E a segunda confirmação deve executar retry limitado se a primeira tentativa falhou antes de persistir
  E o sistema não deve criar transação duplicada, cancelar sem comando explícito nem enviar confirmação duplicada

Cenário: escrita aceita não pode finalizar como sucesso sem recurso financeiro
  Dado que `idempotent_write` retorne `resourceID` vazio para uma confirmação aceita
  Quando o workflow `pending-entry` finalizar o passo de escrita
  Então o step deve terminar como failed ou erro de negócio tipado
  E `PendingStatusCancelled` só pode ser usado para cancelamento explícito, expiração ou substituição, não para falha de escrita aceita
  E `platform_runs.status` não pode ficar `succeeded` para uma escrita aceita sem ledger ou transação

Cenário: run, workflow, ledger, transação e scorer concordam sobre o resultado
  Dado qualquer tentativa de registro financeiro via WhatsApp
  Quando a execução terminar
  Então `platform_runs.correlation_key` deve conter o WAMID da mensagem que abriu ou retomou a execução
  E `workflow_runs.correlation_key` deve localizar a pendência durável
  E `platform_scorer_results` deve reprovar tool-call accuracy quando não houver efeito financeiro persistido
  E dashboards ou consultas operacionais devem diferenciar sucesso persistido, replay, cancelamento, expiração e falha

Cenário: erro de persistência de run não é ignorado
  Dado uma pendência que abre um `platform_run`
  Quando a atualização final do run falhar
  Então o erro deve ser logado com `run_id`, WAMID, workflow e stage
  E deve existir métrica agregada para falha de update de run
  E a jornada não deve ser declarada saudável até a inconsistência ser sanada ou reconciliada

Cenário: status outbound do WhatsApp fica rastreável
  Dado uma resposta enviada para a cliente
  Quando a WhatsApp Cloud API publicar status de entrega
  Então `whatsapp_message_status` deve armazenar `message_id`, `recipient_id`, status e erro quando houver
  E a ausência de status deve ser consultável como "status não recebido" em vez de ser confundida com sucesso ou falha de envio

Cenário: identidade canônica evita dependência silenciosa do legado
  Dado uma cliente ativa resolvida por `users.whatsapp_number`
  Quando a principal for estabelecida com sucesso
  Então deve existir ou ser criado de modo idempotente o vínculo canônico em `user_identities`
  E a trilha de `auth_events` deve indicar se a resolução foi por identidade canônica, backfill ou legado

Cenário: logs, traces e métricas reconstroem a jornada sem expor conteúdo sensível
  Dado qualquer inbound WhatsApp financeiro
  Quando a jornada passar por handler, dispatcher, outbox, consumer, runtime, workflow, tool e repository
  Então logs estruturados e spans devem carregar WAMID, `run_id`, `thread_id` opaco, workflow, status, outcome, duração e erro sanitizado
  E métricas devem permanecer agregadas por `agent_id`, workflow, step, status, outcome e tool
  E nenhuma métrica deve usar `user_id`, telefone, email ou texto da mensagem como label

Cenário: regressão produtiva vira golden test
  Dado a conversa produtiva desta cliente entre 2026-07-09 00:02 UTC e 2026-07-09 00:18 UTC
  Quando a implementação for validada em CI
  Então deve existir golden set cobrindo ativação, personalização de orçamento, "Gastei 10 na padaria no dinheiro", "Gastei 19 na padaria no Pix", "Hoje", "Sim" e repetição de "Sim"
  E os testes devem provar ausência de falso múltiplo lançamento, ausência de orçamento padrão indevido, ausência de confirmação duplicada e presença de transação final

Cenário: validação Go e Mastra bloqueia entrega incompleta
  Dado alteração em Go nos módulos `internal/agents`, `internal/platform`, `internal/budgets`, `internal/categories` ou integração com Postgres
  Quando a história for implementada
  Então devem passar `go build`, `go vet`, `go test -race -count=1` no escopo alterado e `golangci-lint run` quando disponível
  E devem existir testes unitários, integração ou e2e para workflows duráveis, idempotência, scorers, outbox e WhatsApp inbound
  E handlers, consumers, jobs e tools devem continuar adapters finos delegando para use cases
```

## Dados e Permissões
- Dados obrigatórios: `user_id`, WAMID inbound, `thread_id` WhatsApp, `platform_thread_id`, `run_id`, `workflow_run_id`, `outbox_event_id`, texto inbound no armazenamento operacional autorizado, status do workflow, status do run, tool calls, ledger key, transaction id, orçamento, alocações, cartão, scorers e erro sanitizado.
- Perfis/permissões: cliente autenticada via WhatsApp só acessa os próprios dados financeiros; workers e servidores usam credenciais de serviço; operadores técnicos podem consultar DB/logs/traces/métricas com finalidade de suporte e devem mascarar telefone/email em relatórios externos.
- Privacidade: métricas não devem incluir telefone, email, texto livre ou `user_id` como labels; a reconstrução usuário-específica deve ocorrer por DB/traces/logs com acesso restrito e trilha auditável.

## Dependências
- `internal/platform/whatsapp` para assinatura, dedup, dispatcher, status webhook e rate limit.
- `internal/identity` para `EstablishPrincipal`, entitlement ativo, `auth_events` e vínculo canônico de identidade.
- `internal/agents` para `AgentRuntime`, `WhatsAppInboundConsumer`, resumer chain, pending-entry workflow, budget creation workflow, tools financeiras, guard chain e scorers.
- `internal/platform/{agent,memory,workflow,tool,scorer}` para Thread, Run, WorkingMemory, PendingStep, tool-calling e workflow durável no estilo Mastra.
- `internal/budgets`, `internal/categories`, `internal/card` e `internal/transactions` para orçamento, alocação, categorização, cartão, ledger e transação.
- Postgres com `outbox_events`, `platform_threads`, `platform_messages`, `platform_runs`, `workflow_runs`, `workflow_steps`, `platform_scorer_results`, `agents_write_ledger`, `transactions`, `budgets`, `budgets_allocations`, `whatsapp_message_status`, `user_identities` e `auth_events`.
- OpenRouter como provider LLM único já conectado; a feature não deve introduzir fallback chain.
- OTel LGTM, Prometheus, Tempo, logs Docker e health checks para evidência operacional agregada.

## Fora de Escopo
- Trocar provider LLM, criar fallback de LLM ou reescrever o substrato `internal/platform`.
- Criar dashboard visual novo; a história exige dados rastreáveis e pode ser atendida por métricas, traces, logs e consultas existentes ou incrementais.
- Alterar preço, billing, Kiwify, reconciliação de assinatura ou regra comercial de entitlement.
- Criar novas categorias financeiras fora da taxonomia vigente, salvo mapeamento necessário para corrigir a categorização da própria jornada.
- Abrir ticket em ferramenta externa ou publicar PR automaticamente.

## Evidências
- Entrada: solicitação para recuperar os últimos 7 dias e focar somente no usuário `3140d64a-6010-4464-8bb6-cf8be1b10035`, WhatsApp informado e email informado, usando SSH, logs, tracing, métricas e DB.
- Base de código: `cmd/server/whatsapp_wiring.go:27` encadeia dedup, principal, limiter, outbox e agent route; `internal/platform/whatsapp/dispatcher/dispatcher.go:104` processa inbound, stale webhook, dedup e resolução de principal; `internal/identity/application/usecases/establish_principal.go:222` tenta `user_identities` e depois legado `users.whatsapp_number`; `internal›agents›module.go:233` cria `AgentRuntime` com write tools; `internal›agents›module.go:347` publica `agents.whatsapp.inbound.v1`; `internal›agents›infrastructure›messaging›database›consumers›whatsapp_inbound_consumer.go:190` tenta pending-entry, destructive, card, budget e onboarding antes do agente; `internal›agents›application›usecases›register_attempt.go:102` usa chave de pendência por usuário e thread; `internal›agents›application›workflows›pending_entry_workflow.go:361` aceita confirmação e chama escrita; `internal›agents›application›workflows›pending_entry_workflow.go:551` transforma erro de idempotência em step failed; `internal›agents›application›workflows›pending_entry_workflow.go:555` conclui step com `PendingStatusCancelled` quando `resourceID` vem vazio; `internal›agents›application›usecases›pending_entry_continuer.go:260` abre run com WAMID; `internal›agents›application›usecases›pending_entry_continuer.go:306` ignora erro de update do run; `internal/platform/agent/runtime.go:107` preenche `CorrelationKey` com `MessageID`; `internal›agents›application›tools›register_expense.go:56` exige apenas `amountCents` e `description` no schema; `internal›agents›application›workflows›budget_creation_decisions.go:50` valida soma de distribuição em 10000 basis points; `internal›agents›application›workflows›budget_creation_workflow.go:273` cria e ativa orçamento; `internal›agents›application›agents›guard_chain.go:46` executa pre e post guards com métrica.
- Produção DB: usuário criado em `2026-07-09 00:02:24 UTC`, assinatura e entitlement ativos, 11 `auth_events` `principal_established`, 11 WAMIDs em `channel_processed_messages`, 43 outbox events do usuário publicados integralmente, uma thread `53ff12d4-df2d-4dac-b3cf-843c26a26a2a`, 20 mensagens em `platform_messages`, 4 `platform_runs` do agente como `succeeded/routed` com `correlation_key` vazio, 1 workflow de onboarding sucedido, 1 workflow `pending-entry` sucedido com estado final cancelado, 0 linhas em `agents_write_ledger`, 0 transações e 0 status outbound em `whatsapp_message_status`.
- Produção conversa: a cliente enviou objetivo financeiro, renda, cartão, recusou a distribuição padrão, enviou uma distribuição personalizada com Custo Fixo 2500, Conhecimento 0, Prazeres 500, Metas 0 e Liberdade Financeira 2000, negou recorrência, tentou registrar "Gastei 10 na padaria no dinheiro", tentou registrar "Gastei 19 na padaria no Pix", respondeu "Hoje", respondeu "Sim" e repetiu "Sim".
- Produção orçamento: o orçamento `521e630a-e99e-45d1-82dc-58b7d4f11d2c` ficou ativo para `2026-07` com total `500000`, mas as alocações persistidas foram padrão em basis points `4000/1000/1000/1000/3000`, divergindo da distribuição personalizada enviada.
- Produção registro de despesa: após as confirmações, não houve ledger nem transação; o workflow `pending-entry` reteve descrição `padaria`, pagamento `pix`, valor `1900`, data textual `hoje`, resposta "Não consegui registrar. Tente novamente em breve." e estado final cancelado.
- Produção scorers: para a primeira despesa, `tool-call-accuracy=0`; para a segunda despesa inicial, `tool-call-accuracy=0`; para "Hoje", `tool-call-accuracy=0`; para o primeiro "Sim", `tool-call-accuracy=1`, embora ledger e transação tenham permanecido vazios.
- Logs: filtro por `user_id`, telefone mascarável, WAMIDs e termos de pending-entry encontrou apenas `onboarding.activation.phone_matched` para o usuário; erros recorrentes de `billing-reconciliation` por Kiwify 401 apareceram no período, mas não há evidência de impacto direto nesta jornada.
- Métricas e tracing: Prometheus expõe séries como `agent_runs_total`, `agents_whatsapp_inbound_total`, `agents_pending_entry_total`, `agents_write_total`, `workflow_runs_total`, `whatsapp_dispatcher_route_total` e `outbox_dead_letter_total`; Tempo retorna traces de serviços `mecontrola-api` e `mecontrola-worker`, mas a reconstrução usuário-específica depende hoje de DB/outbox/runs porque métricas não carregam `user_id`, e os runs normais observados não carregaram WAMID em `correlation_key`.
- Inferências: a causa raiz provável é combinação de gaps de domínio e observabilidade: personalização de orçamento não é tratada como contrato durável de confirmação, a pendência de despesa permite estado final cancelado para escrita aceita sem recurso, a resposta ao usuário pode se desacoplar do efeito persistido, e os scorers medem tool invocation sem provar persistência.
- Não evidenciado: não foi encontrada prova de status outbound WhatsApp para este destinatário nos últimos 7 dias; não foi encontrada linha em `user_identities` para esse usuário/telefone; não foi encontrada transação financeira persistida para as confirmações da cliente; não foi encontrado log de erro específico do pending-entry para o WAMID da falha.

## Notas de Validação
- Skills aplicadas: `$user-stories` para formato backlog e validação; `$go-implementation` para confronto de módulo Go, arquitetura, gates e `go.mod`; `$mastra` para Thread, Run, tools, workflows, memory, scorer e WhatsApp inbound; `$domain-modeling-production` para comandos, eventos, estados, invariantes e erros; `$design-patterns-mandatory` para gate anti-overengineering.
- Decisão de design pattern: `select_pattern.py` retornou `status=reject`; a US exige solução direta, refactor local e invariantes nos workflows/adapters existentes, sem novo pattern GoF primário.
- Comandos de domínio modelados: `PersonalizarOrcamento`, `AtivarOrcamento`, `IniciarRegistroDespesa`, `ConfirmarRegistroDespesa`, `RepetirConfirmacao`, `RegistrarStatusOutbound` e `EstabelecerIdentidadeCanal`.
- Eventos esperados: `OrcamentoPersonalizadoConfirmado`, `OrcamentoAtivado`, `DespesaPendenteCriada`, `DespesaRegistrada`, `RegistroRepetidoIdempotente`, `RegistroFalhou`, `MensagemWhatsAppRespondida`, `StatusWhatsAppRecebido` e `IdentidadeCanalVinculada`.
- Invariantes centrais: sucesso financeiro requer efeito durável; cancelamento exige intenção, expiração ou substituição; replay não duplica; scorer de escrita exige persistência; correlação por WAMID deve existir nos runs; personalização confirmada não pode ser sobrescrita por padrão.
- Drift registrado: `go.mod` declara Go `1.26.5`; o script citado pela skill `scripts/verify-go-mod.sh` não existe no repositório; a validação de histórias existe em `.agents/skills/user-stories/scripts/validar-historias-usuario.py`.
- A implementação desta US deve adicionar ou ajustar testes de regressão com a jornada real dos últimos 7 dias, rodar gates Go proporcionais ao escopo alterado e manter handlers/consumers/tools como adapters finos.
