# Documento de Requisitos do Produto (PRD) — Orquestração Conversacional Confiável do Agente MeControla

<!-- spec-version: 1 -->

> Fonte: `docs/us/2026-07-09-us-orquestracao-conversacional-confiavel-agente-mecontrola.md` (US-001).
> Governança vinculante para techspec e implementação downstream: `go-implementation`, `mastra`,
> `domain-modeling-production`, `design-patterns-mandatory`, além de `AGENTS.md` e das regras hard
> `R-AGENT-WF-001`, `R-WF-KERNEL-001`, `R-ADAPTER-001`, `R-TESTING-001`, `R-DTO-VALIDATE-001`.

## Visão Geral

O agente financeiro MeControla no WhatsApp já tem substrato sólido — Thread, Run, tools, workflows
duráveis, memória e scorers sobre `internal/platform/{agent,workflow,memory,tool,scorer}`, consumido
por `internal/agents`. O problema não é ausência de arquitetura: é que **regras críticas de segurança
conversacional vivem hoje num prompt monolítico** (`internal/agents/application/agents/mecontrola_agent.go:17-250`,
~234 linhas com ~20 regras P0) e dependem da probabilidade do LLM para não falhar. A produção confirma
o custo disso: em 7 dias, 23 runs (19 `succeeded/routed`, 4 `failed/usecaseError`) e scorers médios
baixos — tool-call-accuracy 0,304, completeness 0,149, categorization 0,565.

Esta iniciativa **transforma regras críticas de roteamento, preenchimento de campos, confirmação,
anti-alucinação, fallback e avaliação em uma cadeia explícita, observável e testável antes e depois
da chamada LLM**, preservando a fluidez conversacional, os contratos públicos atuais e todas as
funcionalidades já entregues. O prompt deixa de ser a única defesa e passa a ser reforço de linguagem;
o comportamento crítico passa a ser garantido por código determinístico com teste correspondente.

O padrão primário autorizado é **Chain of Responsibility** (cadeia de guardas conversacionais); o
padrão **State foi rejeitado** como primário porque os workflows de estado já existem e o problema
central é encadear checks antes/depois do LLM, não reintroduzir máquina de estado.

## Objetivos

- **Remover a dependência probabilística do prompt em pontos críticos**: cada regra de segurança
  conversacional hoje descrita apenas nas `instructions` passa a existir como guarda, roteador,
  workflow ou validação de tool, com teste unitário ou golden correspondente.
- **Elevar a qualidade conversacional medida** contra a linha base produtiva coletada, sem regressão
  de contrato público, privacidade, observabilidade ou fluidez:
  - taxa de `failed`/`usecaseError` menor que a linha base (4 em 23 runs), medida com amostra mínima
    e margem (ver RF-51/RF-52);
  - tool-call-accuracy acima de 0,304 (métrica **redefinida** para considerar só runs onde uma tool
    era esperada — ver RF-42);
  - completeness comportamental acima de 0,149 (baseline mantida por continuidade; ver RF-40);
  - categorization acima de 0,565.
- **Fechar gaps operacionais do runtime** hoje silenciosos: resposta vazia, truncamento por length,
  erro de append de mensagem, erro de update de run, run sem scorer, run sem mensagem persistida e
  tool de escrita com erro passam a gerar métrica, log estruturado e critério de alerta.
- **Reduzir custo e latência** evitando a chamada LLM quando um guard determinístico puder responder
  com segurança (multi-item, confirmação pendente, cancelamento explícito, expiração, dados
  obrigatórios ausentes conhecidos pela tool).
- **Fechar dívida de governança** dos identificadores prefixados com `_` em Go de produção dentro de
  `internal/agents/application/workflows/`, que violam a regra hard R5.26.
- **Tornar regressões detectáveis** por golden set versionado + scorers comportamentais + agregados
  produtivos, com gate de deploy em dois níveis.

### Critério de sucesso primário (composto)

O caminho só é declarado **production-ready monitorado** quando, cumulativamente:
1. o gate **pré-deploy** passa — golden set determinístico + scorers + harness real-LLM ≥ 0,90 nos
   cenários-chave (RF-46, RF-47);
2. o gate **pós-deploy** confirma melhora objetiva sobre a linha base com amostra mínima e sem novo
   alerta crítico de privacidade, truncamento ou escrita duplicada (RF-51, RF-52, RF-53);
3. nenhuma funcionalidade existente regride (RF-54..RF-57);
4. a dívida de governança underscore está fechada no escopo afetado (RF-48).

## Histórias de Usuário

- **Como usuário do WhatsApp**, quero que múltiplos gastos numa mesma mensagem sejam tratados com
  segurança (um de cada vez) sem o agente inventar registros, para não gerar lançamentos duplicados
  ou errados.
- **Como usuário**, quero perguntar "como estou indo?" ou "qual foi minha última transação?" e receber
  dados reais e atualizados (consultados por tool), não uma resposta de memória, para confiar nos números.
- **Como usuário**, quero que toda escrita financeira passe por confirmação e seja idempotente, para
  que repetir uma mensagem ou confirmar duas vezes não crie registros duplicados.
- **Como usuário**, quero que uma compra no cartão use o cartão que eu citei (nunca um inventado) e que
  o agente peça escolha quando não reconhecer o apelido, para não lançar na fatura errada.
- **Como usuário**, quero citar "junho" sem ano e o agente perguntar o ano em vez de assumir, e ver
  meses por extenso em português, para não olhar competência errada.
- **Como usuário**, quero que falhas de tool ou do LLM virem uma resposta curta e honesta ("não
  consegui agora"), nunca uma confirmação de sucesso inventada, para não achar que registrei algo que
  não registrei.
- **Como operador técnico**, quero decidir promover ou reverter uma versão do agente por evidência
  (runs, outcomes, scorers, truncamento, escrita duplicada) sem ler o conteúdo das mensagens dos
  usuários, para operar com segurança e privacidade.

## Funcionalidades Core

1. **Cadeia de guardas conversacionais (Chain of Responsibility)** — handlers pequenos, ordenados,
   observáveis e testáveis executados antes e depois do LLM, absorvendo o `MultiItemGuard` atual como
   primeiro handler e cobrindo as demais regras P0 hoje presas no prompt.
2. **Roteamento como contrato verificável** — cada intenção financeira material tem caso golden com
   mensagem de entrada, tool esperada, argumentos esperados, outcome esperado e resposta/propriedade
   verificável.
3. **Scorers comportamentais** — além dos 3 scorers atuais (mantidos por continuidade de baseline),
   um conjunto que mede comportamento real: tool esperada, argumentos obrigatórios, verbatim, formato
   WhatsApp, ausência de alucinação, ausência de termos internos, resposta não vazia, escrita não
   duplicada e competência de mês correta.
4. **Robustez operacional do runtime** — tratamento explícito e observável de resposta vazia,
   truncamento por length, erros de persistência de mensagem/run e erros de tool de escrita.
5. **Endurecimento de workflows e pendências** — pending entry, onboarding, confirmação destrutiva,
   cadastro de cartão e criação de orçamento resistindo a retomada pós-deploy, expiração, cancelamento,
   mensagem repetida, WAMID duplicado, concorrência e replay idempotente.
6. **Gate de qualidade em dois níveis** — pré-deploy (golden set + scorers + real-LLM ≥ 0,90) e
   pós-deploy (agregados produtivos com amostra mínima e critério explícito de rollback).

## Requisitos Funcionais

### Grupo A — Cadeia de guardas conversacionais (Chain of Responsibility)

- RF-01: O sistema DEVE prover uma cadeia de guardas conversacionais no padrão Chain of Responsibility,
  composta por handlers pequenos, ordenados, observáveis e testáveis, executáveis antes e depois da
  chamada LLM, preservando a chamada pública `BuildMeControlaAgent` e a composição atual do módulo.
- RF-02: O `MultiItemGuard` atual DEVE ser absorvido como o **primeiro handler** da cadeia unificada,
  preservando comportamento determinístico, `ToolOutcomeClarify` e a mensagem verbatim atuais, sob
  teste de regressão que prove equivalência de saída.
- RF-03: Cada regra crítica hoje descrita apenas nas `instructions` DEVE existir como guarda, roteador,
  workflow ou validação de tool com teste unitário ou golden correspondente; a `instruction` do agente
  permanece como reforço de linguagem, não como única defesa do comportamento.
- RF-04: A cadeia DEVE bloquear determinísticamente múltiplos lançamentos na mesma mensagem antes do
  LLM, respondendo a orientação verbatim de lançamento único e sem chamar `register_expense`,
  `register_income`, `create_budget` ou qualquer outra tool de escrita, registrando outcome `clarify`
  (ou equivalente auditável) sem conteúdo sensível.
- RF-05: A cadeia DEVE reconhecer o padrão brasileiro de valor (ponto como separador de milhar, ex.:
  `R$ 1.234,56`) e NÃO tratá-lo como dois valores distintos (regressão coberta por golden).
- RF-06: Cada handler da cadeia DEVE expor o resultado de sua decisão (passou / curto-circuitou /
  delegou) de forma observável e determinística, permitindo auditar qual guarda tratou a mensagem.

### Grupo B — Roteamento determinístico e anti-alucinação

- RF-07: O agente DEVE, para consultas financeiras, chamar as tools exigidas pela matriz C1–C7 antes de
  responder e usar **exclusivamente** dados retornados pelas tools na resposta final.
- RF-08: Em follow-up financeiro, o agente DEVE reinvocar a tool correspondente em vez de responder de
  memória.
- RF-09: O agente NÃO PODE afirmar sucesso, valor, categoria ou status sem retorno real de tool
  (`isReplay=true` não conta como novo registro); esta invariante DEVE ser garantida por código
  (guard/validação de tool/runtime), não apenas pelo prompt.
- RF-10: A resposta ao usuário DEVE ser natural, curta, brasileira e pronta para WhatsApp (asterisco
  simples, sem `**` duplo; emoji onde o padrão atual exige), sem termos internos (`workflow`, `thread`,
  `run`, `correlation`, `infraestrutura`, `sistema interno` e equivalentes).

### Grupo C — Escrita financeira, confirmação e idempotência

- RF-11: Toda ação financeira de escrita DEVE passar por tool e, quando aplicável, por workflow de
  confirmação; o fluxo `adapter → tool → usecase` DEVE ser preservado (sem SQL, regra de negócio ou
  branching de domínio em adapter/tool — R-ADAPTER-001, R-AGENT-WF-001.2).
- RF-12: Quando uma tool de escrita retornar `outcome=clarify` com mensagem de confirmação, a resposta
  enviada ao usuário DEVE ser **exatamente** o campo `message`/`clarifyPrompt`/`confirmationPrompt`
  retornado pela tool (verbatim), garantido por guard/validação, não só por instrução.
- RF-13: Uma confirmação posterior do usuário DEVE ser resolvida pelo workflow pendente correspondente,
  sem nova chamada LLM de escrita duplicada.
- RF-14: Uma repetição idempotente (mesmo WAMID / mesma intenção já processada) DEVE informar a
  confirmação sem criar novo registro financeiro, produzindo apenas um efeito financeiro válido.
- RF-15: O estado de espera de qualquer pendência DEVE ser persistido no `Snapshot` do kernel **antes**
  de o agente pedir a clarificação/confirmação, e retomado por merge-patch antes de qualquer parse
  (R-AGENT-WF-001.7).

### Grupo D — Cartão e proveniência de `cardId`

- RF-16: Para compra ou consulta de fatura em cartão, o agente DEVE chamar `resolve_card` antes de
  `register_expense`/`create_recurrence`/`query_card_invoice`, e usar **exclusivamente** o `cardId`
  retornado por `resolve_card` ou `list_cards`.
- RF-17: O agente NÃO PODE usar `cardId` fabricado a partir do texto do usuário; a proveniência do
  `cardId` (originado de `resolve_card`/`list_cards`) DEVE ser garantida por guarda/validação
  determinística, não apenas pelo prompt.
- RF-18: Quando `resolve_card` retornar `found=false`, o agente DEVE pedir escolha ao usuário (sem
  criar cartão automaticamente).

### Grupo E — Competência de mês / `monthRefKind`

- RF-19: Ao citar um mês por nome sem ano, o agente DEVE enviar `monthRefKind=named_without_year` com
  `month` preenchido e `year` ausente nas tools `query_month`/`query_plan`/`create_budget`.
- RF-20: O agente NÃO PODE inferir ano indevidamente; a classificação de `monthRefKind`
  (`current`/`previous`/`next`/`explicit`/`named_without_year`/`unknown`) DEVE ser um conjunto fechado
  (state-as-type), e o agente DEVE repassar verbatim o `clarifyPrompt` quando a tool pedir o ano.
- RF-21: Toda exibição de competência na resposta final DEVE usar mês por extenso em português (ex.:
  "junho de 2026").

### Grupo F — Robustez operacional do runtime

- RF-22: Quando uma tool de escrita ou consulta retornar erro, resposta vazia ou truncamento por
  length, o run DEVE terminar `failed` (ou com outcome de erro compatível) e o WhatsApp DEVE receber
  fallback seguro, curto e sem detalhe técnico, sem confirmação de sucesso/valor/categoria/status
  inventado.
- RF-23: O runtime DEVE **detectar e tratar** truncamento por length (`finish_reason=length`) hoje
  ignorado: o sinal `TruncatedByLength` propagado pela camada LLM DEVE ser consultado no runtime,
  gerar métrica e log estruturado e resultar em run `failed` + fallback seguro (falha-segura).
- RF-24: O teto de tokens do agente (`MaxTokens`, hoje 1536) DEVE ser elevado para reduzir truncamento
  falso em respostas longas legítimas (ex.: resumos C1–C7), mantendo a falha-segura de RF-23 quando o
  truncamento ainda ocorrer. O valor final é definido na techspec.
- RF-25: Erro de `MessageStore.Append` (mensagem não persistida) DEVE gerar métrica e log estruturado
  (hoje apenas `Warn` silencioso, sem métrica); o critério de alerta correspondente DEVE existir.
- RF-26: Erro de `RunStore.Update` (fechamento de run) DEVE ser observado — hoje é engolido com `_ =`;
  DEVE gerar métrica e log estruturado, e as métricas de run NÃO PODEM reportar sucesso quando o
  update falhou.
- RF-27: Quando múltiplas tools falharem no mesmo run, o erro registrado NÃO PODE se limitar
  silenciosamente à primeira; a evidência operacional DEVE permitir identificar as falhas relevantes.
- RF-28: Run sem scorer executado e run sem mensagem persistida DEVEM ser observáveis (métrica + log)
  e cobertos por critério de alerta.

### Grupo G — Scorers e observabilidade de qualidade

- RF-29: Os 3 scorers atuais (`tool-call-accuracy`, `completeness`, `categorization`) DEVEM ser
  **mantidos** para continuidade das baselines coletadas (0,304 / 0,149 / 0,565).
- RF-30: Um conjunto de checagens/scorers comportamentais DEVE ser adicionado, medindo comportamento
  verificável e não presença de palavras genéricas: `expected_tool`, `required_args`, `no_hallucination`,
  `verbatim_required`, `whatsapp_format`, `no_internal_terms`, `no_empty_answer`, `no_duplicate_write`
  e `month_reference_correctness`.
- RF-31: A promoção/rollback de versão DEVE usar **ambos** os conjuntos (atuais + comportamentais) como
  sinal.
- RF-32: A observabilidade DEVE permitir decisão operacional sem ler conteúdo sensível: `run_id`,
  `agent_id`, `status`, `outcome`, `stage`, `tool`, duração, erro sanitizado, `scorer_id`, `score`,
  `workflow` e estado de pendência DEVEM estar disponíveis em log, métrica, trace ou consulta operacional.
- RF-33: Nenhuma métrica pode carregar label de alta cardinalidade — `user_id`, `thread_id`,
  `resource_id`, `correlation_key` ou conteúdo de mensagem são proibidos como label (R-TXN-004,
  R-AGENT-WF-001.5, R-WF-KERNEL-001.4).
- RF-34: Os resultados de avaliação DEVEM ser rastreáveis por `run_id` sem expor a mensagem do usuário
  em métrica.

### Grupo H — Golden set e harness de avaliação

- RF-35: DEVE existir um golden set versionado no repositório cobrindo, no mínimo: registro de despesa,
  registro de receita, consultas C1–C7, cartões, orçamento, recorrências, onboarding, pendências
  conversacionais, confirmações, follow-up, erro de tool, ambiguidade, formato WhatsApp e ausência de
  termos internos.
- RF-36: Cada caso golden de intenção financeira material DEVE declarar mensagem de entrada, tool
  esperada, argumentos esperados, outcome esperado e resposta esperada ou propriedade verificável da
  resposta.
- RF-37: O golden set DEVE ser composto por casos **sintéticos curados** mais casos **derivados de
  incidentes reais reescritos e anonimizados** (sem PII, sem WAMID/`resourceId`/`threadId` reais);
  conteúdo verbatim de produção NÃO PODE ser versionado.
- RF-38: A avaliação DEVE medir, por versão do agente: tool-call accuracy, completude, categorização,
  taxa de falha, duração p95 e truncamento.

### Grupo I — Gate production-ready em dois níveis

- RF-39: O gate **pré-deploy** (bloqueante) DEVE consistir em: golden set determinístico (LLM
  mockado/fixtures) + testes de guard/scorer + harness real-LLM ≥ 0,90 nos cenários-chave.
- RF-40: O CI padrão (por-PR) DEVE rodar apenas a avaliação **determinística**; o harness **real-LLM**
  DEVE rodar sob tag/manual/nightly e obrigatoriamente como gate pré-deploy, para evitar custo e
  flakiness por-PR mantendo o gate real antes de produção.
- RF-41: O deploy DEVE bloquear quando qualquer threshold acordado do gate pré-deploy cair abaixo da
  linha base aprovada.
- RF-42: A métrica `tool-call-accuracy` DEVE ser **redefinida** para considerar apenas runs onde uma
  tool era esperada (excluindo clarify/chat legítimos), evitando gate sobre ruído; a baseline 0,304 é
  reinterpretada sob a nova definição e documentada.
- RF-43: O gate **pós-deploy** DEVE monitorar os agregados produtivos com critério explícito de
  rollback (falhas, scorers, truncamento, escrita duplicada) e amostra mínima (ver RF-51).

### Grupo J — Dívida de governança

- RF-44: Os identificadores prefixados com `_` em Go de produção dentro de
  `internal/agents/application/workflows/` (ex.: `_defaultDistributionBP`, `_welcomeGoalPrompt`,
  `_goalReprompt`, `_goalValueReprompt`, `_incomePrompt`, `_incomeReprompt`, `_cardsReprompt`,
  `_summaryReprompt`, `_conclusionRecurrencePrompt`, `_allocationInputSystemPrompt`,
  `_goalWithValueSystemPrompt`, `_goalValueSystemPrompt` em `onboarding_workflow.go` e seus usos em
  `budget_creation_workflow.go`) DEVEM ser renomeados para a forma idiomática camelCase sem `_`,
  fechando a violação hard R5.26, sem alterar comportamento.

### Grupo K — Endurecimento de workflows e pendências

- RF-45: Pending entry, onboarding, confirmação destrutiva, cadastro de cartão e criação de orçamento
  DEVEM produzir apenas um efeito financeiro válido e responder com texto determinístico para sucesso,
  cancelamento, expiração e repetição idempotente, cobrindo: retomada após deploy, expiração,
  cancelamento, mensagem repetida, WAMID duplicado, concorrência e replay idempotente.
- RF-46: Após efetivar/cancelar/expirar, o run DEVE completar (`Succeeded`/`Failed`), nunca permanecer
  `Suspended` (limpeza determinística; sem draft órfão) — os reapers existentes cobrem o housekeeping.

### Grupo L — Estados fechados (state-as-type) e economia de LLM

- RF-47: Os estados de fronteira DEVEM permanecer tipos fechados (state-as-type), nunca string livre:
  `agent.ToolOutcome`/`agent.RunStatus`/`agent.AwaitingKind`, `workflow.RunStatus`/`StepStatus`/
  `SuspendReason`, `scorer.ScorerKind`, `memory.MessageRole` e o outcome de truncamento introduzido por
  RF-23.
- RF-48: O sistema DEVE evitar a chamada LLM quando um guard determinístico puder responder com
  segurança — em especial multi-item, confirmação pendente, cancelamento explícito, expiração e dados
  obrigatórios ausentes já conhecidos pela tool — reduzindo custo e latência.

### Grupo M — Thresholds produtivos e contrato de regressão

- RF-49: A linha base produtiva de referência é: 19 runs `succeeded`, 4 `failed`, tool-call-accuracy
  média 0,304, completeness média 0,149, categorization média 0,565 (7 dias, 23 runs).
- RF-50: A nova versão DEVE demonstrar melhora contra a linha base: menos falhas que 4/23, tool-call
  accuracy (redefinida) acima de 0,304, completeness acima de 0,149, categorization acima de 0,565, sem
  aumento de truncamento, escrita duplicada, resposta vazia ou falha silenciosa de persistência de
  mensagem/run.
- RF-51: O gate pós-deploy DEVE exigir **amostra mínima** (janela/volume mínimo acordado, ex.: N ≥ 100
  runs ou janela ≥ 14 dias — valor final na techspec/runbook) antes de promover ou reverter, evitando
  decisão sobre ruído estatístico.
- RF-52: A decisão de manter, reverter (rollback) ou promover a versão DEVE ser tomada por evidência
  operacional rastreável por `run_id`, não por impressão subjetiva.
- RF-53: Nenhum novo alerta crítico de privacidade, truncamento ou escrita duplicada pode aparecer após
  o deploy da nova versão.
- RF-54: Nenhuma tool existente pode ser removida, renomeada, ocultada ou ter contrato/schema/outcome
  alterado sem decisão explícita em história própria.
- RF-55: O contrato público de `BuildMeControlaAgent`, `AgentRuntime`, `RunStore`, `ThreadGateway`,
  `MessageStore`, `WorkingMemory`, os schemas strict das tools e os workflows duráveis já conectados
  DEVEM ser preservados.
- RF-56: Os fluxos existentes (registro de despesa/receita, consulta mensal, orçamento, fatura, última
  transação, busca de transações, cartões, recorrências, categorias, onboarding, pendências,
  confirmação destrutiva, criação de cartão, criação de orçamento, memória, scorers e entrega WhatsApp)
  DEVEM continuar cobertos por teste automatizado, golden set ou evidência operacional equivalente.
- RF-57: A mudança só é aceitável quando o comportamento atual suportado por tools, workflows, memory,
  scorers e WhatsApp continuar coberto por teste ou evidência operacional equivalente (contrato de
  regressão).

## Experiência do Usuário

Fluxos observáveis pelo usuário final no WhatsApp (todos em português, curtos, sem termos internos):

- **Multi-item**: mensagem com dois gastos → orientação de lançamento único (verbatim), sem registrar
  nada.
- **Consulta**: "como estou indo?" → dados reais do mês consultados por tool, mês por extenso.
- **Escrita com confirmação**: despesa completa → pergunta de confirmação verbatim da tool → "sim" →
  registro único idempotente; repetir → informa que já está confirmado, sem duplicar.
- **Cartão**: "comprei no Nubank" → `resolve_card` → se não achar, pergunta qual cartão; nunca lança em
  cartão inventado.
- **Mês sem ano**: "quanto gastei em junho?" → pergunta o ano (verbatim), não assume.
- **Falha**: tool/LLM falha ou trunca → "não consegui agora, pode tentar de novo?" (curto, honesto),
  nunca "registrei ✅" falso.

Fluxos observáveis pelo operador técnico (sem conteúdo sensível): runs, tool calls, outcomes, scorers,
workflows pendentes, truncamento e falhas sanitizadas em log/métrica/trace/consulta.

## Restrições Técnicas de Alto Nível

- **Padrão primário: Chain of Responsibility** (decisão de `design-patterns-mandatory` referenciada na
  US). Alternativa mais simples quase escolhida: manter guards como decorators isolados (rejeitada
  porque não organiza a ordem/observabilidade dos múltiplos checks pré/pós-LLM). Padrão **State
  rejeitado** como primário (workflows de estado já existem; reintroduzi-lo duplicaria responsabilidade).
  Padrão complementar só se indispensável na techspec, sem duplicar responsabilidade.
- **State-as-type obrigatório** (`domain-modeling-production`, DMMF): outcomes, status, estados de
  espera e o novo outcome de truncamento são tipos fechados com constantes enumeradas; estados ilegais
  bloqueados por invariante/smart constructor, nunca string livre.
- **Governança hard vinculante para techspec/implementação**: `go-implementation`, `mastra`,
  `R-AGENT-WF-001` (roteamento por registry, tool fina, LLM só nas call-sites sancionadas, Run
  auditável, pending step antes de clarify), `R-WF-KERNEL-001` (kernel genérico, sem domínio),
  `R-ADAPTER-001` (adapters finos, zero comentários em Go de produção), `R-TESTING-001` (testify/suite
  canônico), `R-DTO-VALIDATE-001` (Validate em input DTO).
- **Provider único**: OpenRouter via `internal/platform/llm`, sem fallback chain — fora de escopo criar
  outro provider.
- **Kernel imutável**: a evolução NÃO reescreve o substrato `internal/platform/{agent,memory,workflow,
  tool,scorer}`; consome-o.
- **Privacidade**: conteúdo de mensagem de produção não aparece em dashboard de eficiência nem em
  fixtures versionadas sem política explícita; o agente só acessa dados financeiros do próprio
  `resourceId`.
- **Custo/latência do harness real-LLM**: executado sob tag/manual/nightly e pré-deploy, não por-PR.

## Fora de Escopo

- Trocar provider LLM, criar fallback chain ou adicionar outro vendor de LLM.
- Reescrever o substrato `internal/platform/{agent,memory,workflow,tool,scorer}`.
- Alterar schema estrutural do Postgres sem história própria de persistência.
- Criar recomendação bancária, de investimento, empréstimo, seguro ou imposto complexo fora do domínio
  financeiro pessoal do MeControla.
- Publicar ticket em Jira, Azure DevOps ou GitHub Issue nesta entrega.
- Remover, renomear ou degradar qualquer funcionalidade existente sem história própria.

## Suposições e Questões em Aberto

Decisões travadas nas rodadas de esclarecimento (todas com recomendação aceita):

1. **Scorers**: manter os 3 atuais (continuidade de baseline) e adicionar os 9 comportamentais como
   sinal independente; promoção/rollback usa ambos (RF-29..RF-31).
2. **Truncamento**: falha-segura (run `failed` + fallback) **e** elevar o teto de `MaxTokens` para
   reduzir truncamento falso (RF-23, RF-24).
3. **Gate**: dois níveis — pré-deploy local bloqueante + pós-deploy monitorado com rollback (RF-39,
   RF-43).
4. **Golden set**: sintéticos curados + incidentes reais reescritos/anonimizados; nada verbatim de
   produção (RF-37).
5. **Harness real-LLM**: determinístico no CI por-PR; real-LLM sob tag/nightly + pré-deploy (RF-40).
6. **Cadeia de guardas**: absorver `MultiItemGuard` como primeiro handler da cadeia unificada (RF-02).
7. **Gate produtivo robusto**: amostra mínima + margem + redefinição de `tool-call-accuracy` (RF-42,
   RF-51).

Suposições explícitas (a resolver na techspec, não bloqueiam o PRD):

- **A1**: o valor final do novo teto de `MaxTokens` (hoje 1536) é definido na techspec com base em
  medição de tamanho real de resposta dos resumos C1–C7.
- **A2**: o mecanismo interno de cada handler da cadeia (pré vs pós-LLM, ordem exata, ponto de wrap em
  relação a `BuildMeControlaAgent`) é desenhado na techspec, preservando o contrato público.
- **A3**: o N exato da amostra mínima e a margem por métrica do gate pós-deploy são fixados na
  techspec/runbook.
- **A4**: a proveniência determinística de `cardId` (RF-17) pode ser garantida por guarda de contexto,
  por validação de tool que rejeite `cardId` não originado de `resolve_card`/`list_cards`, ou por
  combinação; a técnica é decidida na techspec (o requisito de produto é a garantia, não o mecanismo).
- **A5**: a dívida underscore (RF-44) é tratada **dentro desta entrega**; caso a techspec justifique
  removê-la para tarefa técnica acoplada, isso deve ser explícito e não pode adiar o critério
  production-ready.

Nenhuma questão material permanece aberta para redação; o PRD está pronto para `create-technical-specification`.
