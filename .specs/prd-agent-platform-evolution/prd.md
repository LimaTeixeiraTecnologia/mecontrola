# Documento de Requisitos do Produto (PRD) — Evolução da Plataforma de Agentes (inspirada em Mastra)

<!-- spec-version: 1 -->

## Visão Geral

O `internal/agent` do Me Controla já é um agente conversacional de finanças pessoais em
WhatsApp/Telegram com base sólida: parser LLM na fronteira (`ParseInbound`), roteamento
`IntentWorkflow → Tool → binding → usecase`, Thread/Run auditável, WorkingMemory no system
prompt, ObservationMemory assíncrona e pending step (clarificação de categoria). O
`internal/platform/workflow` já entrega um **kernel genérico de workflow** (Step, `then`/`branch`/
`parallel`, suspend/resume durável, retry, housekeeping, estados como tipos fechados), concluído
em `prd-workflow-kernel`.

Este PRD define a **próxima evolução incremental** da plataforma de agentes, adotando — de forma
seletiva e justificada — conceitos do [Mastra](https://github.com/mastra-ai/mastra) que agregam
valor real ao Me Controla, **sem** reproduzir o framework e **sem** reintroduzir complexidade
desnecessária. Três capacidades foram priorizadas com o solicitante:

1. **Execução composta determinística (plano multi-tool)** — uma única mensagem pode disparar uma
   sequência de ações (ex.: "paguei 50 no mercado e quanto gastei esse mês?" → registra + relatório),
   executada de forma determinística sobre o kernel, **sem** loop de raciocínio LLM.
2. **Human-in-the-Loop (HITL) para ações destrutivas/sensíveis** — confirmação humana explícita
   antes de efetivar operações de alto impacto (deletar/editar último lançamento, reconfigurar
   budget, deletar cartão), reusando o suspend/resume durável do kernel e o audit trail existente.
3. **Recuperação contextual e memória** — entregar o contexto certo no momento certo via
   recuperação **estruturada** (sem infra vetorial): histórico/padrões do próprio usuário e
   taxonomia de categorias; mais resumo de histórico conversacional longo e expansão estruturada da
   memória observacional.

### Decisão arquitetural central — "autônomo" sem loop LLM

O solicitante pediu "capacidade de agente autônomo", mas confirmou **manter o single-turn
determinístico** (sem LLM decidindo iterativamente quais tools chamar). Diante das restrições
vigentes — `R-AGENT-WF-001.4` (LLM apenas no step de parse) e o não-goal P2-1 (multi-turn ao LLM
é não-goal por determinismo) — a recomendação **aceita** é reinterpretar "autônomo" como
**plano determinístico multi-tool**: o `ParseInbound` extrai um **plano ordenado de 1..N intents**;
um workflow executor roda o plano via kernel (sequência + branch + retry como "avaliação" pura),
com short-circuit em falha dura e agregação determinística das respostas. Isso entrega a semântica
Mastra de "planejar → executar → avaliar → encerrar ao atingir o objetivo/condição de parada"
**sem** quebrar determinismo, custo ou previsibilidade, e **sem** violar nenhuma regra hard.

## Objetivos

- **Composição determinística (DX + UX)**: permitir que uma mensagem componha múltiplas ações sem
  novo `case intent.Kind` no switch de domínio e sem LLM no meio da execução, reaproveitando o
  kernel e os IntentWorkflows/Tools existentes.
- **Segurança de operações sensíveis (HITL)**: nenhuma operação destrutiva/sensível é efetivada sem
  confirmação humana explícita, com estado persistido, retomada exata e auditoria da intervenção.
- **Contexto certo na hora certa**: aumentar a qualidade das respostas e reduzir clarificações com
  recuperação estruturada de histórico do usuário e taxonomia de categorias, sem armazenar/indexar
  dados desnecessários e sem nova infraestrutura.
- **Memória de longo prazo controlada**: histórico longo cabe no contexto via resumo, e a memória
  observacional evolui de texto livre para estrutura versionada — **apenas** onde houver caso de uso
  real.
- **Não regressão (guardrail)**: todo fluxo single-intent atual preserva comportamento idêntico
  (mesmas respostas e outcomes), com suíte verde e gates `R-*` passando.

### Métricas-chave a acompanhar

- Nº de novos `case intent.Kind` no switch de `daily_ledger_agent.go` por capacidade entregue:
  **deve ser 0**.
- % de mensagens compostas (≥2 ações) executadas corretamente em ordem com agregação determinística:
  alvo **100%** nos cenários de aceite.
- Operações destrutivas/sensíveis efetivadas **sem** confirmação humana: **0**.
- Resume bem-sucedido de gate HITL após restart simulado: **100%** em teste de durabilidade.
- Redução na taxa de `OutcomeClarify` de categoria após recuperação contextual de taxonomia:
  alvo de **redução mensurável** (linha de base medida antes/depois).
- Diferença de comportamento (respostas/outcomes) em fluxos single-intent antes vs depois:
  **0**.
- Cardinalidade de métrica: **sem** `user_id`/`category_id`/`correlation_key` como label
  (herda R-TXN-004 / R-AGENT-WF-001.5 / R-WF-KERNEL-001.4).

## Histórias de Usuário

- Como **usuário do Me Controla**, quero mandar "paguei 50 no mercado e quanto gastei esse mês?" e
  receber as duas respostas (registro confirmado + resumo), para resolver tudo em uma mensagem.
- Como **usuário**, quero que, ao pedir "apaga o último lançamento", o assistente me peça
  confirmação antes de remover, para eu não perder dados por engano.
- Como **usuário**, quero que o assistente lembre dos meus padrões ("você costuma gastar ~R$ 600/mês
  em mercado") e use a taxonomia de categorias para classificar melhor, reduzindo perguntas
  repetidas.
- Como **engenheiro do `internal/agent`**, quero compor um plano multi-tool registrando passos
  reutilizáveis no seam, com **zero** novo `case intent.Kind`, reaproveitando IntentWorkflows e o
  kernel.
- Como **engenheiro**, quero expressar um gate de aprovação humana como suspend/resume do kernel,
  para reusar durabilidade, idempotência e auditoria já existentes em vez de inventar mecanismo
  novo.
- Como **operador/SRE**, quero observabilidade por capacidade (plano executado, gate aprovado/negado,
  recuperação acionada) com cardinalidade controlada, para diagnosticar sem explodir métricas.
- Como **revisor**, quero que cada capacidade preserve as regras hard vigentes
  (`R-AGENT-WF-001`, `R-WF-KERNEL-001`, `R-ADAPTER-001`, `R-TESTING-001`) sem flexibilização.

## Funcionalidades Core

### A. Execução composta determinística (plano multi-tool)

- **O que faz**: permite que uma mensagem produza um **plano ordenado de 1..N intents** executado em
  sequência determinística, com short-circuit em falha dura e agregação das respostas em uma única
  resposta ao usuário.
- **Por que importa**: é o padrão de maior valor para um assistente financeiro em chat; entrega a
  sensação de "agente que planeja e age" sem custo/risco de loop LLM.
- **Como funciona em alto nível**: o `ParseInbound` (único call-site de LLM) passa a poder retornar
  um plano (lista ordenada de intents já tipados e determinísticos); um **IntentWorkflow executor**
  roda cada passo via os IntentWorkflows/Tools existentes sobre o kernel; "avaliar resultado
  intermediário" e "condição de parada" são **regras puras** (branch/short-circuit), nunca LLM.
  Plano de 1 intent é exatamente o comportamento atual (não regressão).

### B. Human-in-the-Loop para ações destrutivas/sensíveis

- **O que faz**: intercepta operações de alto impacto e suspende a execução aguardando confirmação
  humana explícita; ao confirmar, retoma e efetiva; ao cancelar/expirar, descarta sem efeito.
- **Por que importa**: lançamentos comuns seguem report-only sem fricção (decisão vigente), mas
  ações destrutivas/sensíveis precisam de proteção contra erro/intenção ambígua.
- **Como funciona em alto nível**: reusa o **suspend/resume durável** do kernel (mesmo mecanismo do
  pending step de categoria), persistindo o estado de espera com tipo fechado de "aguardando
  aprovação"; a retomada ocorre antes do `ParseInbound`, espelhando o padrão atual; toda intervenção
  humana (aprovado/negado) é registrada no audit trail (decision/run).

### C. Recuperação contextual e memória

- **O que faz**: injeta no contexto, quando relevante, (1) **histórico/padrões do próprio usuário**
  (ex.: média por categoria) e (2) **taxonomia de categorias**; sumariza histórico conversacional
  longo; e estrutura a memória observacional.
- **Por que importa**: melhora qualidade das respostas e reduz clarificações sem inflar custo nem
  exigir nova infraestrutura.
- **Como funciona em alto nível**: recuperação é **query estruturada** no Postgres existente via
  `binding → usecase` (sem RAG vetorial), exposta ao `ContextBuilder` para compor o system prompt;
  o resumo de histórico reaproveita o pipeline LLM já existente da ObservationMemory; a expansão da
  memória observacional torna preferências/comportamentos recorrentes estruturados e versionados.

## Requisitos Funcionais

### A. Execução composta determinística (plano multi-tool)

- RF-01: O `ParseInbound` DEVE poder produzir um **plano ordenado de 1..N intents** já tipados e
  determinísticos; um plano de 1 intent DEVE ser comportamentalmente idêntico ao fluxo atual.
- RF-02: A execução do plano DEVE ser **determinística** — proibido invocar LLM, prompt rendering ou
  fallback chain durante a execução dos passos (preserva R-AGENT-WF-001.4).
- RF-03: O executor de plano DEVE rodar cada passo pelos IntentWorkflows/Tools existentes sobre o
  kernel, **sem** adicionar novo `case intent.Kind` ao switch de `daily_ledger_agent.go`
  (preserva R-AGENT-WF-001.1).
- RF-04: O executor DEVE aplicar **short-circuit** em falha dura de um passo de escrita (não executar
  passos subsequentes dependentes) e DEVE **agregar** as respostas dos passos executados em uma
  única resposta determinística ao usuário.
- RF-05: O resultado e o outcome de cada passo do plano DEVEM ser **auditáveis** como parte do Run
  (status, outcome por passo), sem `user_id`/`category_id` como label de métrica.
- RF-06: A condição de parada ("objetivo atingido" / falha terminal) DEVE ser expressa como
  **regra pura** (decisão determinística sobre o estado), nunca como avaliação por LLM.
- RF-07: O comportamento de mensagens single-intent DEVE permanecer **idêntico** (não regressão
  verificável por testes).

### B. Human-in-the-Loop (HITL) para ações destrutivas/sensíveis

- RF-08: As operações **deletar último lançamento**, **editar último lançamento**, **reconfigurar
  budget** e **deletar cartão** DEVEM exigir **confirmação humana explícita** antes de efetivar.
- RF-09: O gate HITL DEVE **suspender** a execução persistindo o estado de espera de forma durável,
  reusando o suspend/resume do kernel (mesmo mecanismo do pending step), sem depender de contexto em
  memória do processo.
- RF-10: A retomada do gate DEVE ser **idempotente** e sobreviver a restart/crash: reprocessar a
  mesma confirmação NÃO DEVE duplicar o efeito; cancelar/expirar NÃO DEVE produzir efeito algum.
- RF-11: O estado de espera do gate DEVE usar **tipo fechado** (state-as-type) para "aguardando
  aprovação" — nunca `string` livre em assinatura pública (alinha AwaitingKind/DMMF).
- RF-12: Toda intervenção humana (aprovado/negado/expirado) DEVE ser registrada no **audit trail**
  (decision/run), com correlação ao Run e à operação alvo.
- RF-13: A retomada do gate DEVE ocorrer **antes** do `ParseInbound` (espelhando
  `continuePendingExpenseConfirmation`), e o estado DEVE ser **limpo** imediatamente após
  efetivação/cancelamento — sem draft órfão.
- RF-14: O gate HITL NÃO DEVE introduzir fricção em lançamentos comuns (expense/income/card purchase
  report-only permanecem sem confirmação — preserva a decisão vigente).

### C. Recuperação contextual e memória

- RF-15: A recuperação de conhecimento DEVE usar **query estruturada** no Postgres existente via
  `binding → usecase` — **proibido** RAG vetorial, pgvector ou store de vetor dedicado neste escopo.
- RF-16: A recuperação DEVE expor ao `ContextBuilder`, quando disponível: (a) **histórico/padrões do
  próprio usuário** (ex.: agregados por categoria) e (b) **taxonomia de categorias**; ausência de
  dados NÃO é erro.
- RF-17: O conteúdo recuperado DEVE entrar no **system prompt** do `ParseInbound` (preserva
  R-AGENT-WF-001.8: working memory/contexto no system prompt), sem chamar LLM fora do parse.
- RF-18: O sistema DEVE **resumir** histórico conversacional longo para caber no contexto sem perder
  informação relevante, reaproveitando o pipeline LLM assíncrono existente (ObservationMemory).
- RF-19: A memória observacional DEVE evoluir para registrar **preferências/comportamentos
  recorrentes de forma estruturada e versionada**, mantendo o gatilho assíncrono e o limite de
  retenção atuais.
- RF-20: A recuperação e a memória DEVEM ser **escopo `resource` (por `user_id`)** quando aplicável,
  isoladas por usuário, sem vazar dados entre usuários/canais indevidamente.

### Governança e qualidade (transversal)

- RF-21: Nenhuma capacidade DEVE adicionar `case intent.Kind` de domínio ao switch de
  `daily_ledger_agent.go` (R-AGENT-WF-001.1); comportamento novo entra como
  IntentWorkflow/Tool/passo de kernel.
- RF-22: Tools e passos DEVEM permanecer **adapters finos**: sem regra de negócio, SQL direto ou
  branching de domínio (R-AGENT-WF-001.2 / R-ADAPTER-001.2); LLM apenas no parse (R-AGENT-WF-001.4).
- RF-23: Outcomes e estados novos (ex.: outcome de plano, "aguardando aprovação") DEVEM ser **tipos
  fechados** (R-AGENT-WF-001.3 / DMMF state-as-type).
- RF-24: O consumo do kernel pelo agent DEVE preservar a fronteira `R-WF-KERNEL-001`: o kernel
  permanece genérico (sem import de domínio); semântica Thread/Run/WorkingMemory/PendingStep
  permanece exclusiva de `internal/agent`.
- RF-25: Toda métrica nova DEVE ter **cardinalidade controlada** (labels apenas de enums fechados;
  proibido `user_id`/`category_id`/`correlation_key`).
- RF-26: Cada capacidade DEVE passar nos gates `R-ADAPTER-001`, `R-AGENT-WF-001`, `R-WF-KERNEL-001`,
  `R-TESTING-001` e nos checklists `R0–R7` da skill `go-implementation`.
- RF-27: Toda implementação Go DEVE ter **zero comentários** em produção (R-ADAPTER-001.1), sem
  `init()` (R0), sem `panic` em produção (R5.12), `context.Context` em toda fronteira de IO (R6),
  `errors.Join`/`fmt.Errorf %w` (R7).

## Experiência do Usuário

- **Mensagem composta**: usuário envia uma frase com mais de uma intenção; recebe uma resposta única
  e coerente que reflete cada ação executada, na ordem, com falha parcial sinalizada de forma clara
  (o que foi feito e o que não foi).
- **Confirmação de ação sensível**: ao pedir uma operação destrutiva/sensível, o usuário recebe uma
  pergunta de confirmação curta ("Confirma apagar o último lançamento de R$ X?"); responde
  sim/não; só então o efeito acontece. Resposta ambígua mantém o gate aberto; cancelamento/expiração
  encerra sem efeito.
- **Respostas contextuais**: quando relevante, as respostas usam padrões do próprio usuário e a
  taxonomia para classificar/explicar melhor, reduzindo perguntas repetidas de categoria.
- **Acessibilidade/canais**: comportamento idêntico em WhatsApp e Telegram; mensagens curtas e
  determinísticas, alinhadas ao runbook conversacional vigente.

## Restrições Técnicas de Alto Nível

- **Stack**: Go (versão conforme `go.mod`); toda implementação segue a skill `go-implementation`
  (Etapas 1–5 + checklist R0–R7), obrigatória e inegociável.
- **Localização**: capacidades vivem em `internal/agent`; o kernel genérico em
  `internal/platform/workflow` é **consumido**, não estendido com domínio. Recuperação estruturada
  acessa o Postgres existente via `binding → usecase`.
- **Determinismo**: nenhuma capacidade introduz loop de raciocínio LLM; LLM permanece exclusivo do
  `ParseInbound` (e das exceções já sancionadas: fallback conversacional e onboarding).
- **Sem nova infraestrutura para RAG**: proibido pgvector/store vetorial/serviço externo neste
  escopo; recuperação é query estruturada no banco atual.
- **Durabilidade/idempotência**: gates HITL e planos suspensíveis reusam suspend/resume durável do
  kernel + idempotência por `event_id` + lock otimista; sem goroutine leak, shutdown cooperativo.
- **DMMF / state-as-type**: estados/outcomes novos são tipos fechados; proibidos `Result[T,E]`
  customizado, currying, DSL de pipeline e monads (anti-padrões hard).
- **Privacidade**: histórico e memória são escopo por usuário; recuperação não cruza dados entre
  usuários; redação/sanitização do audit trail preserva o padrão existente.
- **Observabilidade**: stack `otel-lgtm`; métricas com cardinalidade controlada; cada capacidade é
  auditável como parte do Run.
- **Governança (gate)**: aderência integral às regras hard vigentes
  (`R-AGENT-WF-001`, `R-WF-KERNEL-001`, `R-ADAPTER-001`, `R-TESTING-001`, `R-DTO-VALIDATE-001`,
  `R-TXN-WORKFLOWS-001` quando tocar transactions); qualquer nova regra/addendum necessário é gate
  na techspec/ADR antes do código.

## Fora de Escopo

- **Loop de agente autônomo com LLM** (plan→act→observe→iterate dirigido por LLM, tool-calling
  iterativo): explicitamente excluído — mantém-se single-turn determinístico (não-goal P2-1).
- **RAG semântico/vetorial**: pgvector, embeddings, store de vetor dedicado e busca semântica de
  base de conhecimento do produto (FAQ) ficam fora; podem ser reavaliados em PRD futuro se houver
  caso de uso comprovado.
- **Gate HITL para lançamentos comuns** (expense/income/card purchase report-only): permanecem sem
  confirmação por decisão vigente.
- **Framework HITL genérico no kernel** além do necessário para as 4 operações destrutivas/sensíveis:
  o mecanismo reusa suspend/resume existente; generalização ampla é trabalho futuro.
- **Novos primitivos de control-flow no kernel** (loops `foreach`/`dountil`, `map`, workflows
  aninhados, streaming por step, scorers, `.sleep()`/`.waitForEvent()`): fora deste escopo
  (já listados como futuros em `prd-workflow-kernel`).
- **Reescrita do parser/roteamento de modelos** (OpenRouter/FallbackChain/CircuitBreaker) e mudança
  de comportamento de fluxos single-intent.
- **Detalhes de desenho** (assinaturas, schema de colunas, parâmetros de retenção/limite, ADRs):
  pertencem à Especificação Técnica.

## Decisões Resolvidas (sem questões em aberto)

Resolvidas com o solicitante em 2 rodadas de múltipla escolha:

- **Escopo**: três capacidades — (A) plano determinístico multi-tool, (B) HITL para ações
  destrutivas/sensíveis, (C) recuperação contextual + memória. Workflows com grafo já existem
  (`prd-workflow-kernel`) e **não** são reescritos.
- **"Autônomo"**: reinterpretado como **plano determinístico multi-tool** (recomendação aceita),
  preservando single-turn e R-AGENT-WF-001.4; **sem** loop LLM.
- **HITL**: gate apenas para **deletar último lançamento, editar último lançamento, reconfigurar
  budget e deletar cartão**, reusando suspend/resume do kernel; lançamentos comuns seguem sem gate.
- **RAG**: fontes = **histórico do próprio usuário** + **taxonomia de categorias**; infraestrutura =
  **query estruturada no Postgres existente, sem vetorial**.
- **Memória**: incluir **resumo de histórico longo** e **expansão estruturada da memória
  observacional**, reaproveitando o pipeline assíncrono existente.
- **Não regressão** como guardrail: fluxos single-intent inalterados.

Nenhuma suposição material remanescente. Parametrizações numéricas (limites de passos do plano,
timeout/expiração do gate HITL, retenção/limite de memória, tamanho de resumo) e assinaturas/schema
exatos são **parâmetros da Especificação Técnica**, não decisões de produto pendentes.
