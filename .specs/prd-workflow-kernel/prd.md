# Documento de Requisitos do Produto (PRD) — Workflow Kernel reutilizável

<!-- spec-version: 2 -->

## Visão Geral

Hoje o `internal/agent` chama "Workflow" o que, na prática, é uma **tabela de dispatch plana**
(`composite.go`): um `map[intent.Kind]Tool` com um `WriteGuard` de 4 fases fixas
(Authorize → Replay → Policy → Audit). Não há composição de passos: sequência, ramificação,
paralelismo, IO tipada por passo, retry por passo, observabilidade por passo ou suspend/resume
de primeira classe. O suspend/resume existe, mas é **bespoke e fora do workflow**
(`pendingexpense.Draft`, retomado antes do `ParseInbound`). O `internal/platform` tem kernels
sólidos (outbox, uow, idempotency, worker/jobs, events), mas **nenhum primitivo de orquestração
de workflow**.

Este PRD define um **Workflow Kernel genérico, robusto e reutilizável** em `internal/platform`,
inspirado no modelo do [Mastra](https://github.com/mastra-ai/mastra) (Step, control-flow,
suspend/resume, Run auditável), porém **sem semântica de domínio**. O kernel oferece os primitivos
de orquestração; o `internal/agent` (e, no futuro, outros módulos) os consome mantendo sua
semântica própria. O agent continua dono exclusivo de Thread/Run/WorkingMemory/PendingStep
**semânticos** — o kernel fornece apenas o mecanismo genérico de execução, persistência de estado
e retomada.

O valor: transformar a extensão do agent de "adicionar tool ao dispatch plano" em "compor passos
reutilizáveis", com confiabilidade (resume idempotente, retry por passo, controle de concorrência)
e auditabilidade (observabilidade por passo) de nível produção, sem reintroduzir
`switch case intent.Kind`.

## Objetivos

- **DX (extensão sem switch/case)**: adicionar um fluxo multi-step novo deve exigir apenas o
  registro de passos no seam (`buildRegistry`), com **zero** novo `case intent.Kind` e reaproveitando
  passos já existentes entre workflows.
- **Confiabilidade**: suspend/resume com snapshot **durável**; resume **idempotente** após
  restart/crash (mesma entrada não duplica efeito); controle de concorrência por lock otimista;
  retry por passo com política configurável e falha terminal determinística; zero goroutine leak /
  shutdown cooperativo.
- **Auditabilidade**: 100% dos Runs e Steps persistidos auditáveis (status como tipo fechado,
  `duration_ms`, `attempt`, erro) com observabilidade por passo e cardinalidade de métrica
  controlada (sem `user_id`/`category_id` como label).
- **Eficiência**: snapshot persistido apenas onde agrega valor (escrita/suspensível); leitura pura
  roda in-process sem custo de I/O extra.
- **Reuso real comprovado**: o `pendingexpense.Draft` passa a ser o **primeiro consumidor** do
  suspend/resume genérico, e o write de transactions do agent é migrado como prova production-ready.
- **Não regressão (guardrail)**: o fluxo migrado preserva comportamento idêntico (mesmas respostas e
  outcomes), com suíte de testes verde e gates `R-*` passando.

### Métricas-chave a acompanhar

- Nº de novos `case intent.Kind` introduzidos por fluxo novo: **deve ser 0**.
- Nº de passos reutilizados entre workflows na prova: **≥ 1** (WriteGuard como passos compartilhados).
- Taxa de resume bem-sucedido após restart simulado: **100%** em teste de durabilidade.
- Cobertura de Runs/Steps persistidos com status fechado + `duration_ms`: **100%**.
- Efeitos duplicados sob resume concorrente / entrega dupla: **0**.
- Diferença de comportamento (respostas/outcomes) antes vs depois da migração: **0**.

## Histórias de Usuário

- Como **engenheiro do `internal/agent`**, quero compor um fluxo multi-step (sequência + ramificação
  + paralelismo) registrando passos reutilizáveis, para entregar comportamento novo sem adicionar
  `case intent.Kind` nem duplicar guarda/auditoria.
- Como **engenheiro de plataforma**, quero um kernel de workflow genérico em `internal/platform`,
  sem acoplamento a `intent`/`agent`/`transactions`, para que qualquer módulo possa orquestrar
  passos com as mesmas garantias de durabilidade e observabilidade.
- Como **mantenedor**, quero que um fluxo suspenso (ex.: aguardando esclarecimento de categoria)
  retome do ponto exato após reinício do processo, de forma idempotente e segura sob concorrência,
  para não perder estado nem duplicar lançamentos.
- Como **operador/SRE**, quero observabilidade por passo (status, duração, erro, tentativa) com
  cardinalidade controlada e housekeeping de retenção, para diagnosticar falhas sem explodir
  métricas nem deixar tabelas crescerem indefinidamente.
- Como **revisor**, quero que a separação entre "kernel genérico" e "workflow de intent do agent"
  esteja codificada em regra hard antes do código, para impedir drift e vazamento de regra de
  domínio para o kernel.

## Funcionalidades Core

- **Step composável (unidade reutilizável)**: passo nomeado com IO tipada, encadeável e testável
  isoladamente. É a unidade básica de reuso entre workflows.
- **Control-flow Core (paridade Mastra pragmática)**: sequencial (`then`), condicional (`branch`),
  paralelo (`parallel`) com agregação determinística e cancelamento cooperativo. Loops/map/nested
  e paridade total ficam fora deste escopo.
- **Suspend/Resume de primeira classe**: um passo pode suspender retornando um sinal tipado; a
  retomada recompõe o estado a partir de um snapshot persistido, com idempotência e lock otimista.
  O `pendingexpense.Draft` é migrado para consumir esse mecanismo.
- **Run durável e auditável**: cada execução de escrita/suspensível é um Run com Steps; estado
  persistido em tabelas relacionais do kernel (`workflow_runs` + `workflow_steps`, via uow),
  permitindo resume após restart/crash. Leitura pura roda in-process.
- **Confiabilidade operacional**: retry por passo configurável, falha terminal determinística
  (run `failed` auditável após máximo de tentativas) e job de housekeeping com retenção configurável.
- **Consumo pelo agent + prova de migração**: o write de transactions do agent é migrado para um
  workflow multi-step do kernel, expressando o `WriteGuard` como passos 1:1 e o `record-expense`
  como fluxo real com branch de categoria e suspend/resume de clarificação.

## Requisitos Funcionais

### Kernel genérico (internal/platform)

- RF-01: O kernel de workflow DEVE residir em `internal/platform/workflow` e NÃO DEVE depender de
  pacotes de domínio (`intent`, `agent`, `transactions` ou similares).
- RF-02: O kernel DEVE oferecer `Step` como unidade composável, com entrada/saída tipada e
  identificador único, testável isoladamente.
- RF-03: O kernel DEVE suportar composição **sequencial** de passos (saída de um alimenta o próximo).
- RF-04: O kernel DEVE suportar composição **condicional** (branch): selecionar o próximo passo a
  partir de uma decisão pura sobre o estado corrente.
- RF-05: O kernel DEVE suportar composição **paralela** (parallel) com agregação determinística dos
  resultados e cancelamento cooperativo via `context.Context`.
- RF-06: O kernel DEVE oferecer **suspend/resume** como capacidade de passo: um passo pode suspender
  retornando um sinal/estado tipado, e a execução pode ser retomada do ponto de suspensão.
- RF-07: O estado do Run e dos Steps DEVE ser persistido em **duas tabelas relacionais**
  (`workflow_runs` e `workflow_steps`), gravadas pela uow existente do `internal/platform`, com
  migrations versionadas.
- RF-08: O snapshot durável DEVE ser gravado **apenas para runs de escrita ou suspensíveis**; runs
  de leitura pura DEVEM executar in-process sem persistir snapshot.
- RF-09: O resume DEVE ser **idempotente** e sobreviver a restart/crash do processo: reprocessar a
  mesma entrada NÃO DEVE duplicar efeitos já confirmados.
- RF-10: O kernel DEVE garantir segurança sob **resume concorrente** (entrega dupla / múltiplas
  instâncias) via **lock otimista por versão** na linha do run (compare-and-set, perde a corrida se a
  versão mudou) somado à idempotência por `event_id` existente, sem segurar conexão.
- RF-11: O kernel DEVE suportar **retry por passo** com política configurável (máximo de tentativas e
  backoff), de forma determinística e observável.
- RF-12: Ao esgotar o máximo de tentativas, o run DEVE entrar em **falha terminal**: marcado `failed`
  (status fechado), com erro auditável e métrica emitida; é **proibido** retry infinito.
- RF-13: Cada Run e cada Step persistidos DEVEM ser **auditáveis**, registrando no mínimo: status,
  `duration_ms`, número da tentativa e erro (quando houver).
- RF-14: O kernel DEVE emitir **observabilidade por passo** (trace/span e métrica) com **cardinalidade
  controlada** — proibido `user_id` ou `category_id` como label de métrica.
- RF-15: Os estados do kernel (ex.: `RunStatus`, `StepStatus`, motivo de suspensão) DEVEM ser
  **tipos fechados** (state-as-type), nunca `string` livre em assinatura pública.
- RF-16: A execução paralela e os passos de longa duração DEVEM ser **canceláveis**, com shutdown
  cooperativo e sem goroutine leak.
- RF-17: DEVE existir **job de housekeeping** (reaproveitando `internal/platform/worker`, no padrão do
  reaper de outbox) que purgue runs concluídos após retenção configurável por ambiente.

### Reuso e integração com internal/agent

- RF-18: O `internal/agent` DEVE consumir o kernel mantendo Thread/Run/WorkingMemory/PendingStep
  **semânticos** como responsabilidade exclusiva do agent; o kernel NÃO DEVE redefinir esses
  conceitos de domínio.
- RF-19: O kernel DEVE deter o nome canônico **`Workflow`** em `internal/platform`; o "Workflow"
  atual do agent DEVE ser renomeado para **`IntentWorkflow`** (camada de roteamento de intent sobre o
  kernel), eliminando a ambiguidade de nomes.
- RF-20: O `pendingexpense.Draft` DEVE ser migrado para consumir o suspend/resume do kernel,
  preservando seu contrato atual (`AwaitingKind` e `TransactionKind` permanecem tipos fechados; resume
  antes de `ParseInbound`; limpeza imediata após execução/cancelamento).
- RF-21: A guarda de escrita (`WriteGuard`) DEVE ser expressa como **passos composáveis 1:1** —
  mesmas quatro fases (Authorize → Replay → Policy → Audit), na mesma ordem e com a mesma semântica de
  short-circuit — reutilizáveis entre workflows de escrita.

### Fluxo de prova (migração incremental aditiva)

- RF-22: O write de transactions do agent (record expense/income/card purchase) DEVE ser migrado para
  um workflow **multi-step** do kernel, coexistindo com o modelo atual durante a transição (aditivo,
  sem big-bang).
- RF-23: O `record-expense` DEVE ser expresso como fluxo multi-step **real**, exercitando branch
  (resolução/ambiguidade de categoria) e suspend/resume (clarificação), com a guarda de escrita como
  passos — validando os primitivos de ponta a ponta.
- RF-24: O comportamento do fluxo migrado DEVE permanecer **idêntico** ao atual (mesmas respostas e
  mesmos outcomes), verificável por testes de não regressão.

### Extensibilidade (DX)

- RF-25: Adicionar um fluxo multi-step novo DEVE exigir apenas o registro de passos no seam
  (`buildRegistry`), com **zero** novo `case intent.Kind` no switch de `daily_ledger_agent.go`.
- RF-26: Passos DEVEM ser **reutilizáveis entre workflows** (composição sem duplicação de lógica de
  guarda, auditoria ou formatação).

### Governança

- RF-27: DEVE ser criada uma regra hard `R-WF-KERNEL-001` definindo o kernel genérico em
  `internal/platform`: sem regra de domínio, sem SQL de domínio, estados como tipos fechados,
  cardinalidade controlada.
- RF-28: A regra `R-AGENT-WF-001` (itens .6 e .8) DEVE ser aditada para **distinguir** "kernel
  genérico de workflow" (permitido em platform) de "workflow de intent + Thread/Run/WorkingMemory
  semânticos" (exclusivos de `internal/agent`).
- RF-29: A redação concreta de `R-WF-KERNEL-001` e do addendum de `R-AGENT-WF-001` é **gate
  obrigatório na techspec/ADR — deve ser concluída antes de qualquer código do kernel**.
- RF-30: O LLM DEVE permanecer **proibido dentro de passos de execução** do kernel/agent; LLM apenas
  no step de parse (`ParseInbound`), preservando R-AGENT-WF-001.4.

### Testabilidade e validação

- RF-31: O control-flow puro (sequência/branch) DEVE ser testável **sem mocks de infraestrutura**; os
  passos DEVEM ser testáveis isoladamente.
- RF-32: A prova DEVE passar nos gates `R-ADAPTER-001`, `R-AGENT-WF-001`, `R-TESTING-001` e nos
  checklists `R0–R7` da skill `go-implementation`.

## Restrições Técnicas de Alto Nível

- **Linguagem/stack**: Go (versão conforme `go.mod`). Toda implementação segue a skill
  `go-implementation` (Etapas 1–5 + checklist R0–R7) — obrigatória e inegociável.
- **Localização**: o kernel vive em `internal/platform/workflow`; reutiliza uow, idempotency,
  observabilidade e worker existentes em vez de criar infraestrutura paralela.
- **Persistência**: snapshot em tabelas relacionais `workflow_runs` + `workflow_steps`, escrita via
  uow (escrita = uow + factory; leitura = repositório por DI), migrations versionadas, conforme o
  padrão de database local do projeto.
- **Concorrência/idempotência**: lock otimista por versão no run + idempotência por `event_id`;
  resume e efeitos colaterais não duplicam em reprocessamento; paralelismo cancelável e shutdown
  cooperativo alinhados ao `internal/platform/worker`.
- **Eficiência**: persistência de snapshot restrita a runs de escrita/suspensíveis; leitura pura
  in-process.
- **DMMF / state-as-type**: estados do kernel e do agent são tipos fechados; proibido `Result[T,E]`
  customizado, currying, DSL de pipeline ou monads (anti-padrões hard).
- **Zero comentários em Go de produção** (R-ADAPTER-001.1); **sem `init()` / sem `panic` em produção**
  (R0/R5.12); `context.Context` em toda fronteira de IO (R6); `errors.Join`/`fmt.Errorf %w` (R7).
- **Observabilidade**: stack `otel-lgtm`; métricas com cardinalidade controlada (herda R-TXN-004 /
  R-AGENT-WF-001.5) — labels apenas de enums fechados.
- **Governança (gate)**: criar `R-WF-KERNEL-001` e aditar `R-AGENT-WF-001.6/.8` **antes** do código
  do kernel; a separação kernel-genérico vs workflow-de-intent é pré-condição para o kernel viver em
  platform sem violar regra hard vigente.

## Fora de Escopo

- Loops (`foreach`/`dountil`/`dowhile`), `map` de dados e workflows aninhados — ficam para iteração
  futura (control-flow além do Core).
- Paridade total com Mastra: streaming de eventos por step, scorers, `.sleep()`/`.waitForEvent()`.
- Migração dos demais módulos (`transactions`, `billing`) para o kernel — apenas o consumo pelo agent
  e a prova de migração estão neste escopo; expansão a outros módulos é trabalho posterior.
- Migração big-bang dos 4 workflows atuais (`transactions`/`budget`/`cards`/`conversational`) — apenas
  **um** fluxo de escrita é migrado como prova; os demais migram depois com o mesmo padrão.
- Dead-letter dedicado para runs falhos — falha terminal usa status `failed` auditável; uma fila de
  dead-letter, se necessária, é trabalho futuro.
- Reescrita do parser/LLM ou do roteamento de modelos (OpenRouter/FallbackChain/CircuitBreaker).
- Mudança de comportamento funcional do agent (respostas, outcomes) — a prova é explicitamente de
  não regressão.
- Detalhes de desenho das interfaces/assinaturas, schema exato das colunas e ADRs — pertencem à
  Especificação Técnica.

## Decisões Resolvidas (sem questões em aberto)

Todas as ambiguidades materiais foram resolvidas com o solicitante (4 rodadas de múltipla escolha):

- Fronteira/reuso: **kernel genérico em `internal/platform`**, agent consome mantendo semântica própria.
- Control-flow: **Core** (sequencial + branch + parallel + suspend/resume), retry e observabilidade por passo.
- Suspend/resume: **generalizado**; `pendingexpense.Draft` vira o primeiro consumidor.
- Migração: **aditiva**, uma prova real = write de transactions com `record-expense` multi-step.
- Durabilidade: **tabelas relacionais** `workflow_runs` + `workflow_steps` via uow; snapshot **só** para
  escrita/suspensível; leitura pura in-process.
- Nomenclatura: kernel = **`Workflow`**; agent = **`IntentWorkflow`**.
- WriteGuard: convertido em steps **1:1** (4 fases, ordem e short-circuit idênticos).
- Concorrência: **lock otimista por versão** + idempotência por `event_id`.
- Falha terminal: **máx. tentativas → run `failed`** auditável + métrica; sem retry infinito.
- Housekeeping: **job de retenção configurável** reaproveitando `worker`.
- Governança: **gate** — `R-WF-KERNEL-001` + addendum `R-AGENT-WF-001.6/.8` redigidos na techspec/ADR
  antes do código.
- Critérios de sucesso priorizados: **DX**, **Confiabilidade** e **Auditabilidade**, com não regressão
  como guardrail.

Nenhuma suposição ou questão em aberto remanescente. Parametrizações numéricas (dias de retenção,
máximo de tentativas, curva de backoff) e assinaturas/sintaxe exatas são **parâmetros da techspec**,
não decisões de produto pendentes.
