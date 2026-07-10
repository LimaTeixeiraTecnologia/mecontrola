# Documento de Requisitos do Produto (PRD)

<!-- spec-version: 2 -->

Jornada WhatsApp financeira sem falso sucesso

Fonte: `docs/us/2026-07-10-us-jornada-whatsapp-financeira-sem-falso-sucesso.md` (US-001).
Skills obrigatórias declaradas: `go-implementation`, `mastra`, `domain-modeling-production`, `design-patterns-mandatory`.

## Visão Geral

Clientes ativas do MeControla operam suas finanças por conversa no WhatsApp: ativam a
assinatura, concluem onboarding, cadastram cartão, personalizam o orçamento e registram despesas
com confirmação. A promessa central do produto é de confiança: **cada resposta de sucesso precisa
corresponder exatamente ao estado financeiro efetivamente gravado na conta**.

Na janela produtiva analisada (usuário `3140d64a-6010-4464-8bb6-cf8be1b10035`, 2026-07-03 a
2026-07-10), a jornada quebrou essa promessa em cinco frentes que a análise de código e de produção
confirmou:

1. **Personalização de orçamento perdida** — a cliente enviou uma distribuição personalizada que
   fecha 100% (Custo Fixo R$2500 / Conhecimento R$0 / Prazeres R$500 / Metas R$0 / Liberdade
   R$2000 sobre renda de R$5000), mas o orçamento `521e630a-...` foi ativado com a distribuição
   **padrão** (basis points `4000/1000/1000/1000/3000`).
2. **Despesa simples classificada como múltiplos lançamentos** — "Gastei 10 na padaria no dinheiro"
   recebeu a orientação de "um lançamento por vez". A causa real não foi detecção de múltiplos
   valores, e sim uma pendência anterior ainda ativa (`wf.ErrRunAlreadyExists`) reusando a mesma
   mensagem para dois casos semanticamente distintos.
3. **Confirmação que não vira sucesso** — o "Sim" de confirmação de despesa não gerou ledger nem
   transação; o workflow `pending-entry` finalizou com `StepStatusCompleted` +
   `PendingStatusCancelled` quando a escrita idempotente retornou `resourceID` vazio, respondendo
   "Não consegui registrar. Tente novamente em breve." e encerrando sem recurso financeiro.
4. **Rastreabilidade rompida** — 4 `platform_runs` do agente ficaram `succeeded/routed` com
   `correlation_key` vazio (sem WAMID); o erro de `run.Update` final é engolido; e `user_identities`
   não recebe vínculo canônico no fallback legado, com `auth_events` sem registrar o path de
   resolução.
5. **Scorer com falso positivo** — `tool-call-accuracy=1` para o primeiro "Sim" mesmo com ledger e
   transação vazios, porque o scorer mede invocação de tool, nunca efeito financeiro persistido.

O valor desta feature é eliminar o **falso sucesso** ponta a ponta: nenhuma resposta de sucesso sem
efeito durável, nenhuma perda de personalização, nenhuma confirmação duplicada indistinguível,
nenhum run sem correlação por WAMID, e nenhum scorer que aprove escrita sem persistência — tudo
protegido por um golden set derivado da jornada real e por gates Go e real-LLM.

Todas as 14 tabelas envolvidas (`outbox_events`, `platform_threads`, `platform_messages`,
`platform_runs`, `workflow_runs`, `workflow_steps`, `platform_scorer_results`,
`agents_write_ledger`, `transactions`, `budgets`, `budgets_allocations`, `whatsapp_message_status`,
`user_identities`, `auth_events`) já existem no schema `000001_initial_schema.up.sql`. Esta feature
é de **correção de domínio, idempotência e observabilidade sobre os workflows e adapters
existentes**, não de criação de schema nem de reescrita de plataforma.

## Objetivos

Sucesso é medido por invariantes verificáveis, não por sensação de estabilidade:

- **Zero falso sucesso**: 0 respostas de sucesso financeiro sem o estado durável correspondente
  (orçamento ativado com alocações, ledger, transação, mensagem e outbox quando assíncrono).
- **Zero cancelamento indevido**: 0 escritas aceitas finalizadas como `PendingStatusCancelled` sem
  ledger/transação; `Cancelled` reservado a cancelamento explícito, expiração ou substituição.
- **Zero personalização perdida**: 100% das distribuições personalizadas que fecham 100% persistem
  exatamente os valores confirmados nos casos de teste; 0 sobrescrita silenciosa pela distribuição
  padrão.
- **Correlação completa**: 0 runs financeiros com `correlation_key` vazio; todo run carrega o WAMID
  que o abriu ou retomou.
- **Concordância de estado**: 100% de concordância entre `platform_runs.status`,
  `workflow_runs.status`, `workflow_runs.state.status`, `agents_write_ledger`, `transactions` e
  `platform_scorer_results` numa amostra de auditoria da jornada.
- **Scorer honesto**: 0 aprovações de `tool-call-accuracy` (ou scorer equivalente de escrita) quando
  não há efeito financeiro persistido.
- **Regressão barrada**: golden set da jornada real verde em CI e gate real-LLM `>= 0,90` por
  categoria da jornada.

## Histórias de Usuário

- Como **cliente ativa no WhatsApp**, quero que a distribuição de orçamento que eu personalizei
  seja exatamente a que fica ativa, para não descobrir depois que o sistema usou percentuais padrão
  que eu recusei.
- Como **cliente ativa no WhatsApp**, quero registrar uma despesa simples e receber apenas o pedido
  do dado que falta ou a confirmação final, para não ser tratada como se tivesse enviado vários
  lançamentos.
- Como **cliente ativa no WhatsApp**, quero que "Sim" para confirmar uma despesa realmente grave a
  transação e me diga que registrou, para confiar que a resposta de sucesso é verdadeira.
- Como **cliente ativa no WhatsApp**, quero que, se eu confirmar duas vezes, o sistema não crie
  despesa duplicada nem me faça confirmar de novo sem motivo.
- Como **operador técnico de suporte**, quero reconstruir qualquer jornada financeira por
  `user_id`, `thread_id`, WAMID, `run_id`, `workflow_run_id`, `outbox_event_id` e status
  persistidos, para diagnosticar sem depender de métricas com dados sensíveis.
- Como **responsável pela qualidade do agente**, quero que os scorers reprovem qualquer jornada de
  escrita que não deixou efeito no ledger/transação, para que a métrica de acurácia não me minta.

## Funcionalidades Core

1. **Personalização de orçamento como contrato durável** — a distribuição confirmada pela cliente
   é a que persiste; a distribuição padrão nunca sobrescreve uma personalização válida; distribuição
   que não fecha 100% pede correção e mantém o orçamento pendente com estado de espera tipado.
2. **Pendência de despesa determinística** — uma despesa simples inicia pendência ou registra após
   confirmação; nunca recebe o texto de múltiplos lançamentos; nova despesa com pendência ativa
   recebe mensagem distinta pedindo para concluir a pendência atual.
3. **Confirmação que só declara sucesso com efeito durável** — "Sim" grava exatamente 1 ledger + 1
   transação vinculada ao WAMID original; escrita aceita sem recurso financeiro vira erro tipado, não
   cancelamento; a resposta final reflete o efeito real.
4. **Idempotência por pendência/operação** — repetição de confirmação vira replay se já persistiu ou
   retry controlado se falhou antes de persistir, sem duplicar transação nem confirmar de novo.
5. **Correlação e reconciliação de run** — todo run financeiro carrega o WAMID em
   `correlation_key`; erro de update final é observado (log + métrica) e a jornada não é declarada
   saudável até reconciliar; estados finais de run/workflow/ledger/transação/scorer concordam.
6. **Scorer de persistência** — a acurácia de escrita reprova quando não há efeito financeiro
   persistido e mensagem de sucesso compatível.
7. **Rastreabilidade de status outbound** — ausência de status de entrega WhatsApp é consultável e
   distinguível de falha de envio/persistência.
8. **Identidade canônica com trilha** — resolução por `user_identities` como caminho canônico;
   fallback legado cria vínculo idempotente e registra o path em `auth_events`.
9. **Golden set e gates** — a jornada real vira regressão automatizada com gates Go e real-LLM.

## Requisitos Funcionais

### Orçamento e personalização

- RF-01: Uma distribuição personalizada enviada pela cliente que feche 100% deve ser criada e
  ativada com exatamente os valores/percentuais confirmados; nenhuma alocação padrão pode
  sobrescrever a personalização válida.
- RF-02: Uma distribuição personalizada que não feche 100% não pode ativar orçamento; o fluxo deve
  responder pedindo ajuste objetivo dos percentuais ou valores, manter o orçamento pendente (sem
  ativação parcial) e registrar estado de espera tipado para correção de alocação.
- RF-03: A validação de soma da distribuição deve rejeitar tanto soma menor quanto maior que 100%
  (10000 basis points), de forma consistente nos caminhos de onboarding e de budget-creation, sem
  aceitar distribuição parcial em nenhuma camada.
- RF-04: A decisão completa de orçamento deve ser reconstruível a partir de `workflow_runs`,
  `budgets`, `budgets_allocations`, `platform_messages` e outbox.
- RF-29: A perda de personalização deve ser corrigida nos três vetores identificados, sem depender
  de qual foi a causa isolada em produção: (a) a extração da resposta da cliente deve distinguir
  corretamente entrada em valores (reais) de entrada em percentuais, sem coagir uma na outra; (b)
  quando a cliente enviar uma personalização, a distribuição padrão não pode ser confirmada nem
  aplicada em seu lugar; (c) a validação de soma deve ser simétrica (rejeitar `< 100%` e `> 100%`)
  em ambas as camadas — workflow e domain (`create_budget`) — eliminando a aceitação de soma parcial
  que hoje passa quando `< 10000` basis points.

### Registro de despesa e pendência

- RF-05: Uma mensagem com uma única despesa (um valor, uma descrição, uma forma de pagamento e data
  inferível) deve iniciar uma pendência de registro ou registrar após confirmação, pedindo apenas os
  dados obrigatórios ausentes ou a confirmação final; nunca pode receber a orientação de múltiplos
  lançamentos.
- RF-06: Quando já existir uma pendência de despesa ativa para o mesmo usuário/thread, uma nova
  mensagem de despesa deve receber mensagem distinta informando que há uma despesa aguardando
  confirmação e pedindo para confirmá-la ou cancelá-la primeiro; essa situação não pode reutilizar o
  texto de múltiplos lançamentos.
- RF-07: Ao confirmar uma pendência com dados completos, o sistema deve criar exatamente um registro
  em `agents_write_ledger` e exatamente uma transação vinculada ao WAMID original, ao usuário e à
  operação de registro; a resposta final deve informar sucesso do registro, não uma nova pergunta de
  confirmação.
- RF-08: `platform_messages` deve conter a mensagem inbound e a resposta final da pendência.

### Idempotência e confirmação duplicada

- RF-09: A idempotência do registro é por pendência/operação, não por WAMID isolado: a primeira
  confirmação cria a transação ou registra falha tipada; uma confirmação afirmativa repetida (mesma
  pendência, WAMID diferente) deve responder como replay idempotente se a transação já existir, ou
  executar retry controlado se a tentativa anterior falhou antes de persistir; o sistema nunca pode
  criar transação duplicada, cancelar sem comando explícito nem enviar confirmação duplicada
  indistinguível.
- RF-30: O retry controlado é limitado a uma tentativa por confirmação repetida — cada "Sim"
  subsequente executa no máximo um retry quando a tentativa anterior falhou antes de persistir; se já
  persistiu, é replay. A pendência tem TTL de 30 minutos (alinhado ao TTL de pendência já adotado no
  projeto); após a expiração, a pendência é encerrada como expirada (`PendingStatusExpired`), nunca
  como falha de escrita.

### Ausência de falso sucesso

- RF-10: Se a escrita idempotente aceita retornar `resourceID` vazio, o passo de escrita do workflow
  `pending-entry` deve terminar como `failed` ou erro de negócio tipado; `platform_runs.status` não
  pode ficar `succeeded` para uma escrita aceita sem ledger ou transação.
- RF-11: `PendingStatusCancelled` só pode ser usado para cancelamento explícito do usuário,
  expiração ou substituição; nunca para representar falha de escrita aceita.
- RF-12: Uma resposta de sucesso financeiro só pode ser enviada quando o estado durável
  correspondente existir: orçamento ativado com alocações confirmadas, ledger escrito, transação
  criada, mensagem persistida e outbox publicado quando houver efeito assíncrono.

### Correlação, reconciliação e concordância de estado

- RF-13: `platform_runs.correlation_key` deve conter o WAMID da mensagem que abriu ou retomou a
  execução, para todo run financeiro (rota de agente e retomada de pendência).
- RF-14: `workflow_runs.correlation_key` deve localizar a pendência durável correspondente.
- RF-15: Quando a atualização final de um `platform_run` falhar, o erro deve ser logado com
  `run_id`, WAMID, workflow e stage, e deve existir métrica agregada para falha de update de run; a
  jornada não pode ser declarada saudável enquanto a inconsistência não for sanada ou reconciliada.
- RF-16: `platform_runs.status`, `workflow_runs.status`, `workflow_runs.state.status`,
  `agents_write_ledger`, `transactions` e `platform_scorer_results` devem concordar sobre o
  resultado final entre os estados: sucesso persistido, replay persistido, cancelamento explícito,
  expiração ou falha.

### Scorers e diferenciação operacional

- RF-17: Um scorer de escrita deve reprovar a jornada quando não houver efeito financeiro persistido,
  verificando o efeito final em ledger/transação/orçamento e a compatibilidade da mensagem de
  sucesso; não pode considerar uma jornada de escrita correta apenas porque uma tool foi invocada. A
  verificação deve ocorrer por run (per-run): o efeito de persistência (ledger/transação/orçamento)
  deve ser exposto ao scorer via `RunSample`/metadata, de modo que o próprio run individual seja
  reprovado quando não houve gravação — corrigindo o falso positivo na origem, e não apenas na
  agregação pós-deploy.
- RF-18: Consultas ou dashboards operacionais devem diferenciar sucesso persistido, replay,
  cancelamento, expiração e falha.

### Status outbound do WhatsApp

- RF-19: A ausência de status de entrega outbound deve ser consultável como "status não recebido",
  distinguível de falha de envio ou de falha de persistência; quando a Cloud API publicar status,
  `whatsapp_message_status` deve armazenar `message_id`, `recipient_id`, status e erro quando houver
  (mecanismo já existente deve permanecer funcional e observável).

### Identidade canônica

- RF-20: A resolução de identidade deve funcionar pelo caminho canônico de `user_identities`; quando
  o legado `users.whatsapp_number` for usado com sucesso, o sistema deve criar ou garantir de forma
  idempotente o vínculo canônico em `user_identities`.
- RF-21: A trilha de `auth_events` deve indicar se a resolução da principal foi por identidade
  canônica, backfill ou legado.

### Observabilidade e privacidade

- RF-22: Ao longo de handler, dispatcher, outbox, consumer, runtime, workflow, tool e repository, os
  logs estruturados e spans devem carregar WAMID, `run_id`, `thread_id` opaco, workflow, status,
  outcome, duração e erro sanitizado.
- RF-23: As métricas devem permanecer agregadas por `agent_id`, workflow, step, status, outcome e
  tool; nenhuma métrica pode usar `user_id`, telefone, email ou texto da mensagem como label.
- RF-24: A jornada deve ser reconstruível por `user_id`, `thread_id`, WAMID, `run_id`,
  `workflow_run_id`, `outbox_event_id` e status persistidos, via DB/traces/logs de acesso restrito,
  sem depender de labels de métrica sensíveis.

### Regressão e qualidade

- RF-25: Deve existir um golden set — reprodução anonimizada/sintética da conversa produtiva desta
  cliente (2026-07-09 00:02 a 00:18 UTC) — cobrindo ativação, personalização de orçamento (caso
  válido e caso inválido), "Gastei 10 na padaria no dinheiro", "Gastei 19 na padaria no Pix",
  "Hoje", "Sim" e repetição de "Sim"; os testes devem provar ausência de falso múltiplo lançamento,
  ausência de orçamento padrão indevido, ausência de confirmação duplicada e presença de transação
  final.
- RF-26: Alterações em Go nos módulos `internal/agents`, `internal/platform`, `internal/budgets`,
  `internal/categories` ou na integração Postgres devem passar `go build`, `go vet`,
  `go test -race -count=1` no escopo alterado e `golangci-lint run` quando disponível; devem existir
  testes unitários, de integração ou e2e para workflows duráveis, idempotência, scorers, outbox e
  WhatsApp inbound.
- RF-27: Deve existir gate real-LLM com resultado `>= 0,90` por categoria da jornada antes da
  entrega.
- RF-28: Handlers, consumers, jobs e tools devem permanecer adapters finos delegando para use cases.

## Experiência do Usuário

Fluxos conversacionais afetados (WhatsApp, texto):

- **Onboarding / personalização de orçamento**: a cliente recusa a distribuição padrão e envia a
  sua. Se fecha 100%, o orçamento é ativado com os valores dela e a resposta confirma exatamente
  essa distribuição. Se não fecha, a resposta pede correção objetiva e o orçamento fica pendente.
- **Registro de despesa simples**: "Gastei 10 na padaria no dinheiro" resulta em pendência com valor
  1000, descrição "padaria", pagamento dinheiro, pedindo apenas o que faltar ou a confirmação final
  — nunca a orientação de múltiplos lançamentos.
- **Despesa com pendência já ativa**: em vez do texto de múltiplos lançamentos, a cliente recebe uma
  mensagem clara de que há uma despesa aguardando confirmação e o pedido para confirmar/cancelar
  antes.
- **Confirmação**: "Gastei 19 na padaria no Pix" → "Hoje" → "Sim" grava a transação e responde com
  sucesso real. Um segundo "Sim" responde como replay (sem nova transação), não como nova confirmação
  nem como cancelamento com erro genérico.
- **Falha de escrita**: quando a escrita aceita não deixa recurso financeiro, a cliente recebe uma
  resposta de falha auditável (não um falso sucesso), e o estado interno reflete falha, não
  cancelamento.

## Restrições Técnicas de Alto Nível

- **Sem novo design pattern GoF**: o seletor de `design-patterns-mandatory` retornou `reject`; a
  solução é direta, com refactor local e reforço de invariantes sobre os workflows, tools e adapters
  já existentes — não introduzir novo pattern estrutural/comportamental como solução primária.
- **Substrato Mastra preservado**: usar os primitivos atuais de Thread, Run, WorkingMemory,
  PendingStep, kernel de workflow durável e tool adapter fino; não reescrever `internal/platform`.
- **DMMF state-as-type**: estados de fronteira permanecem tipos fechados enumerados
  (`PendingStatus`, `RunStatus`, `StepStatus`, `SuspendReason`, `OperationKind`, `ToolOutcome`,
  `ScorerKind`), nunca string livre; a falha de escrita aceita deve ser modelada como estado/erro
  tipado.
- **LLM**: OpenRouter é o único provider já conectado; não introduzir fallback chain nem novo
  provider.
- **Cardinalidade de métricas controlada**: labels restritos a enums fechados; proibido `user_id`,
  telefone, email ou texto de mensagem como label (herda R-TXN-004 / R-AGENT-WF-001.5).
- **Adapters finos e zero comentários** (R-ADAPTER-001): handlers/consumers/jobs/producers/tools sem
  regra de negócio, SQL direto ou branching de domínio; código Go de produção sem comentários.
- **Privacidade do golden set**: o golden deve usar reprodução anonimizada/sintética, sem telefone,
  email ou dado pessoal real; reconstrução usuário-específica só por DB/traces/logs restritos.
- **Stack e versão**: Go `1.26.5` (conforme `go.mod`); validação proporcional ao risco do escopo
  alterado.

## Fora de Escopo

- Trocar o provider LLM, criar fallback de LLM ou reescrever o substrato `internal/platform`.
- Criar dashboard visual novo; a rastreabilidade é atendida por métricas, traces, logs e consultas
  existentes ou incrementais.
- Alterar preço, billing, Kiwify, reconciliação de assinatura ou regra comercial de entitlement.
- Criar novas categorias financeiras fora da taxonomia vigente, salvo mapeamento necessário para
  corrigir a categorização da própria jornada.
- Abrir ticket em ferramenta externa ou publicar PR automaticamente.
- **Job de backfill histórico de `user_identities`** para a base legada existente — a correção é
  forward-only (vínculo idempotente na próxima resolução), conforme decisão D-05.
- **Investigar ou corrigir a configuração upstream da WhatsApp Cloud API** para passar a entregar
  status outbound — o escopo cobre apenas tornar a ausência distinguível, conforme decisão D-04.

## Critérios de Sucesso e Gate de Entrega

A feature é considerada pronta quando, cumulativamente:

- Todos os invariantes dos objetivos são satisfeitos (zero falso sucesso, zero cancelamento indevido,
  zero personalização perdida, correlação completa, concordância de estado, scorer honesto).
- O golden set de RF-25 está verde em CI.
- Os gates Go de RF-26 passam no escopo alterado.
- O gate real-LLM de RF-27 atinge `>= 0,90` por categoria da jornada.
- Handlers/consumers/jobs/tools permanecem adapters finos (RF-28) e os gates de governança
  (R-ADAPTER-001, R-AGENT-WF-001, R-TXN-004, R-DTO-VALIDATE-001) retornam limpos.

## Decisões de Produto

Registro das decisões materiais resolvidas com o solicitante (todas com recomendação aceita):

- D-01 — **Escopo**: PRD único cobrindo correção de domínio + observabilidade/identidade + golden,
  como tema coeso "sem falso sucesso".
- D-02 — **Escrita aceita vazia**: desfecho é erro de negócio tipado com `run=failed` e resposta
  auditável de falha; `PendingStatusCancelled` nunca representa falha de escrita (RF-10, RF-11).
- D-03 — **Confirmação duplicada**: idempotência por pendência/operação — 2º "Sim" vira replay se
  persistiu ou retry controlado se falhou antes de persistir (RF-09).
- D-04 — **Status WhatsApp**: escopo é apenas tornar a ausência de status distinguível de falha; a
  entrega de status pela Cloud API é upstream/config e fica fora de escopo (RF-19).
- D-05 — **Identidade canônica**: vínculo idempotente forward no fallback legado + registro do path
  em `auth_events`; sem job de backfill histórico (RF-20, RF-21).
- D-06 — **Despesa com pendência ativa**: mensagem distinta pedindo concluir/cancelar a pendência
  atual; nunca o texto de múltiplos lançamentos (RF-06).
- D-07 — **Gate de sucesso**: invariantes + golden + gates Go + gate real-LLM `>= 0,90` por
  categoria (RF-25, RF-26, RF-27).
- D-08 — **Personalização — vetores de correção**: o PRD exige corrigir os três vetores da perda de
  personalização (extração reais/percent, não confirmação do padrão, validação simétrica em ambas as
  camadas), além de garantir o outcome (RF-29, RF-01, RF-03).
- D-09 — **Scorer de persistência — localização**: a verificação de efeito durável vive no scorer
  per-run, com o efeito de persistência exposto no `RunSample`/metadata; o run individual é reprovado
  quando não houve gravação (RF-17).
- D-10 — **Retry e TTL da confirmação repetida**: no máximo 1 retry por confirmação repetida e TTL
  de pendência de 30 minutos; após expiração, encerra como expirada, nunca como falha de escrita
  (RF-30, RF-09).

## Suposições e Questões em Aberto

Não há questões materiais de produto em aberto e não há suposições pendentes. As dez ambiguidades
identificadas foram resolvidas e registradas como decisões (D-01 a D-10), cada uma com o requisito
funcional correspondente. Detalhes de mecânica de implementação (assinaturas, forma exata de expor a
persistência no `RunSample`, refatorações locais) são deliberados na especificação técnica sem
reabrir decisões de produto — o comportamento esperado já está fixado pelos RFs.
