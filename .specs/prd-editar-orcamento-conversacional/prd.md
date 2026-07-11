# Documento de Requisitos do Produto (PRD) — Editar Orçamento por Conversa (WhatsApp)

<!-- spec-version: 1 -->

> Fonte: `docs/us/2026-07-10-us-editar-criar-orcamento-conversacional.md` (US única, validada).
> Módulos afetados: `internal/agents` e `internal/budgets`.
> Data: 2026-07-10.
> Módulo Go: `github.com/LimaTeixeiraTecnologia/mecontrola`.

## Visão Geral

Assinantes do MeControla planejam suas finanças por um orçamento mensal distribuído em cinco categorias fixas (Custo Fixo, Conhecimento, Prazeres, Metas, Liberdade Financeira). A criação desse orçamento por conversa no WhatsApp já existe e é robusta (tool `create_budget` → workflow durável `budget-creation`). Porém, **editar** um orçamento existente é hoje incompleto: só há ajuste imediato da porcentagem de **uma** categoria (`adjust_allocation`), sem confirmação e apenas sobre orçamento Ativo. Não há caminho conversacional para mudar o valor total nem para refazer a distribuição inteira, e rascunhos (Draft) não podem ser editados.

Esta funcionalidade entrega a **edição conversacional completa do orçamento** pelo WhatsApp — alterar o valor total, ajustar a porcentagem de uma categoria ou refazer a distribuição inteira — sempre com **confirmação humana** antes de aplicar, cobrindo cada caminho de conversa que o runtime já suporta (resolução de mês, clarificação, reprompt, cancelamento, expiração, replay). O valor é permitir que o usuário corrija e replaneje seu orçamento sem recriá-lo do zero e sem risco de mudança acidental.

## Objetivos

- Fechar os gaps de edição G-01..G-05 da US: editar total (G-01), refazer distribuição (G-02), confirmação na edição (G-03), edição de Draft (G-04), robustez com TTL/reaper (G-05).
- Paridade de segurança com a criação: nenhuma mutação de orçamento sem confirmação explícita "sim" e sem falso sucesso.
- Cobrir integralmente as possibilidades de conversa mapeadas (matriz A–F da US), sem inventar respostas fora do runtime.
- Métrica de sucesso (gate de release): **gate real-LLM ≥ 0,90 por categoria** para roteamento da operação e extração de valores, **zero falso-sucesso de escrita** (falha vira `StepStatusFailed` sem recurso persistido) e **cobertura funcional** de cada cenário Gherkin da US.
- Reaproveitar o substrato de plataforma (`internal/platform/{agent,memory,workflow,tool}`) sem reimplementá-lo, respeitando R-AGENT-WF-001 e R-WF-KERNEL-001.

## Decisões Confirmadas (traçabilidade)

| ID | Decisão | Escolha |
|----|---------|---------|
| D-01 | Operações de edição | Editar total **+** ajustar % de 1 categoria **+** refazer distribuição inteira (exclui excluir/resetar) |
| D-02 | Confirmação | Workflow durável com confirmação HITL "sim/não" antes de aplicar |
| D-03 | Criar orçamento | Já implementado — baseline documentado, fora do escopo de desenvolvimento |
| D-04 | Estado alvo | Orçamento Ativo **e** Draft |
| D-05 | Cobertura de conversa | Enumerar cada possibilidade suportada pelo runtime, sem inventar respostas |
| R1 | Competência editável | Qualquer mês com orçamento existente (retroativo, atual e futuro), simétrico ao criar |
| R2 | Draft vazio (auto-draft sem total/alocações) | Rotear para criação (`create_budget`), não editar |
| R3 | Superfície | Apenas conversacional/WhatsApp (sem novos endpoints HTTP) |
| R4 | Gate de sucesso | Real-LLM ≥ 0,90 por categoria + zero falso-sucesso + cobertura funcional |
| R5 | Alertas ao editar | Recalcular no ciclo normal do job existente (sem disparo imediato) |
| R6 | Concorrência | Um fluxo de orçamento por vez por recurso (bloqueia se houver create ou edit pendente) |
| R7 | "Não" na confirmação | Cancela a edição inteira sem efeito (simétrico ao create/destructive-confirm) |
| R8 | Resumo de confirmação | Mostrar o estado novo destacando o que muda |
| E1 | Total abaixo do gasto realizado | Permitir e sinalizar o estouro (não bloquear) |
| E2 | Pedido combinado numa mensagem | Uma operação por fluxo; agente desambigua qual primeiro |
| E3 | Categoria em 0% | Permitido (domínio aceita 0–100) |
| E4 | Cancelamento | Aceito explicitamente em qualquer passo (coleta ou confirmação) |

## Histórias de Usuário

Persona primária: **assinante do MeControla que conversa por texto no WhatsApp**.

- Como assinante, quero alterar o valor total do meu orçamento e confirmar antes de aplicar, para corrigir o planejamento quando minha renda muda, sem recriar tudo.
- Como assinante, quero ajustar a porcentagem de uma categoria e ver como as demais se reequilibram, para redistribuir com segurança.
- Como assinante, quero refazer a distribuição inteira do orçamento de uma vez, para reorganizar minhas prioridades do mês.
- Como assinante, quero editar um orçamento ainda em rascunho, para acertá-lo antes de ativar.
- Como assinante, quero que edições demoradas, repetidas ou canceladas nunca apliquem uma mudança duplicada ou indevida, para confiar no agente.

História única detalhada e critérios de aceite em Gherkin: `docs/us/2026-07-10-us-editar-criar-orcamento-conversacional.md` (US-01).

## Funcionalidades Core

1. **Editar valor total** — coleta o novo total, preserva as porcentagens (basis points) por categoria e recalcula o planejado em centavos; aplica após confirmação.
2. **Ajustar porcentagem de uma categoria** — coleta categoria + porcentagem (0–100), rebalanceia as demais proporcionalmente mantendo soma 100%; aplica após confirmação (endurece o `adjust_allocation` atual com HITL).
3. **Refazer distribuição inteira** — coleta a nova distribuição das cinco categorias (confirmar padrão, percentual ou reais), preserva o total; aplica após confirmação.
4. **Roteamento e desambiguação de operação** — identifica qual operação o usuário quer; pedido vago é desambiguado; pedido combinado trata uma operação por fluxo.
5. **Confirmação humana (HITL) durável** — resumo do estado novo + o que muda, "sim" aplica, "não"/cancelar aborta; ambiguidade gera reprompt único.
6. **Robustez conversacional** — resolução de mês, clarificação de ano, reprompt de valor inválido, TTL/expiração, replay idempotente por `wamid`, um fluxo de orçamento por vez, falha-segura sem falso sucesso.

## Requisitos Funcionais

### Entrada e roteamento
- RF-01: O assinante inicia a edição por linguagem natural no WhatsApp (ex.: "quero mudar meu orçamento").
- RF-02: O agente identifica a operação pretendida (editar total, ajustar categoria, refazer distribuição); pedido vago dispara desambiguação da operação antes de coletar dados.
- RF-03: Diante de pedido combinado numa única mensagem (ex.: total e categoria juntos), o fluxo trata uma operação por vez e o agente confirma qual executar primeiro; a outra é solicitada depois.
- RF-04: O roteamento ocorre por registry/tool (starter de workflow), sem `switch case intent.Kind` (herda R-AGENT-WF-001).
- RF-05: Cada início de edição resolve `Thread → Run` auditável antes de qualquer coleta ou mutação.

### Resolução de competência (mês)
- RF-06: A competência é resolvida a partir de referência de mês (atual, mês passado, próximo mês, mês/ano explícito).
- RF-07: Referência de mês sem ano ("junho") gera clarificação do ano antes de coletar valor.
- RF-08: Referência de mês irreconhecível gera pedido de reformulação, sem prosseguir.
- RF-09: A edição alcança qualquer competência com orçamento existente — retroativa, atual ou futura (simétrico à criação).

### Existência e estado do orçamento
- RF-10: Se não existir orçamento para a competência, o agente oferece criar (não tenta editar).
- RF-11: A edição funciona sobre orçamento Ativo.
- RF-12: A edição funciona sobre orçamento Draft; editar um Draft não o ativa (permanece Draft após a edição).
- RF-13: Se o orçamento na competência for um rascunho vazio/auto-draft (sem total nem alocações definidas), o agente roteia para o fluxo de criação em vez de editar.

### Editar valor total
- RF-14: O fluxo coleta o novo total; valor ≤ 0 ou não identificado gera reprompt sem aplicar mudança.
- RF-15: Ao alterar o total, as porcentagens (basis points) por categoria são preservadas e o planejado em centavos é recalculado por distribuição determinística (arredondamento half-even, ordem canônica).
- RF-16: É permitido definir um total abaixo do valor já gasto no mês; o estouro resultante é sinalizado no resumo/alertas (percentual > 100), sem bloquear a edição.

### Ajustar porcentagem de uma categoria
- RF-17: O fluxo coleta a categoria e a nova porcentagem (0–100 inclusive); porcentagem fora do intervalo é recusada com reprompt.
- RF-18: Nome de categoria irreconhecível gera clarificação entre as cinco categorias válidas.
- RF-19: É permitido ajustar uma categoria para 0%.
- RF-20: As demais categorias são rebalanceadas proporcionalmente, mantendo a soma em 100% (10000 basis points).

### Refazer distribuição inteira
- RF-21: O fluxo coleta a nova distribuição das cinco categorias em três modos: aceitar a sugestão padrão, informar percentuais ou informar valores em reais.
- RF-22: Distribuição inválida (percentuais que não somam 100% ou reais que não somam o total) gera reprompt sem aplicar.
- RF-23: A redistribuição preserva o total atual do orçamento; apenas a distribuição muda.

### Confirmação humana (HITL)
- RF-24: Antes de aplicar qualquer operação, o agente exibe um resumo do estado resultante destacando o que muda e pergunta confirmação ("sim/não").
- RF-25: "sim"/"confirmar"/"ok" aplica a edição; "não"/"cancelar" cancela a edição inteira sem efeito.
- RF-26: Resposta ambígua na confirmação gera um reprompt único; ambiguidade na segunda tentativa cancela a edição sem efeito.
- RF-27: O assinante pode cancelar explicitamente a edição em qualquer passo (durante a coleta ou na confirmação), encerrando o fluxo sem efeito.
- RF-28: Nenhuma mutação de orçamento é persistida sem confirmação explícita do usuário.

### Robustez e idempotência
- RF-29: O estado de espera é persistido de forma durável antes de o agente pedir coleta ou confirmação (fonte única de verdade no snapshot do kernel).
- RF-30: Há TTL de 30 minutos para a espera; após esse prazo o fluxo expira sem efeito e a mensagem do usuário segue para interpretação normal.
- RF-31: Um reaper dedicado limpa fluxos de edição suspensos e abandonados.
- RF-32: Reenvio da mesma mensagem (mesmo `wamid`/identificador) não aplica a edição uma segunda vez (idempotência).
- RF-33: Existe no máximo um fluxo de orçamento por vez por recurso: se já houver criação ou edição pendente, o início de nova edição é bloqueado com aviso de pendência.
- RF-34: Se o serviço de orçamento estiver indisponível no momento de aplicar, o agente informa a indisponibilidade com mensagem específica e não reporta sucesso.
- RF-35: Falha na escrita marca a execução como falha (`StepStatusFailed`), sem persistir recurso e sem reportar sucesso (zero falso-sucesso).

### Alertas
- RF-36: A edição apenas persiste o novo planejado; os alertas de limite (80/90/100%) são reavaliados no ciclo normal do job existente, sem disparo imediato pela edição.

### Segurança e auditoria
- RF-37: Apenas o dono do orçamento pode editá-lo; a escrita é atribuída ao `userID` da identidade do inbound (sem IDOR).
- RF-38: Cada execução é observável como Run auditável (mínimo: `thread_id`, `run_id`, operação, `status`, `error`); métricas usam apenas labels de cardinalidade controlada (proibido `user_id`/`category_id` como label).

### Idioma e conteúdo
- RF-39: A interação ocorre em PT-BR, texto de WhatsApp; respostas novas reaproveitam o tom do fluxo de criação e não inventam mensagens fora do comportamento do runtime.

### Gate de sucesso
- RF-40: O release exige gate real-LLM ≥ 0,90 por categoria (roteamento da operação + extração de valores), zero falso-sucesso de escrita e cobertura funcional de cada cenário Gherkin da US.

## Experiência do Usuário

- Canal único: WhatsApp, texto. O usuário conversa naturalmente; o agente conduz coleta → resumo → confirmação.
- Cada operação termina com um resumo do orçamento resultante (novo total e/ou nova distribuição por categoria), destacando o que mudou, antes da confirmação final.
- Mensagens de erro/clarificação são específicas (mês sem ano, valor não identificado, distribuição inválida, categoria irreconhecível, indisponibilidade) e distintas de um fallback genérico.
- O usuário nunca fica preso: pode cancelar a qualquer momento e o fluxo expira sozinho após 30 minutos de inatividade.

## Restrições Técnicas de Alto Nível

- Consumir o substrato de plataforma existente (`internal/platform/{agent,memory,workflow,tool}`) — kernel de workflow durável com suspend/resume por merge-patch (RFC 7386), TTL/reaper e replay; não reimplementar primitivos (R-AGENT-WF-001, R-WF-KERNEL-001).
- Regra de domínio (recálculo de planejado, rebalanceamento, invariantes total > 0 e soma = 10000) vive em funções `Decide*` puras de `internal/budgets/domain/services`; tools e steps do agente são adapters finos, sem regra de negócio, SQL direto ou branching de domínio (R-ADAPTER-001).
- Estados de fronteira são tipos fechados (state-as-type): status/awaiting/operação do workflow de edição, sem string livre (DMMF).
- Escrita idempotente por identidade do inbound (`wamid`) — sem duplicar mutações em reenvio.
- LLM apenas nas call-sites sancionadas (loop do agente, extração de valores no step, scorer); OpenRouter é o único provider; proibido LLM no kernel.
- Sem novos endpoints HTTP nesta entrega (R3); a superfície é o agente conversacional.
- Persistência exclusivamente via adapters do módulo `budgets` (Postgres); o kernel não conhece SQL de domínio.

## Fora de Escopo

- Criar orçamento por conversa (já implementado; baseline documentado — D-03).
- Excluir ou resetar orçamento (não selecionado em D-01).
- Ativar orçamento (passo separado já existente).
- Alterar total e distribuição na mesma confirmação (são operações distintas neste fluxo — E2).
- Endpoints HTTP/REST para edição (R3).
- Disparo imediato de reavaliação de alertas na edição (R5) e mudanças no motor de alertas.
- Encadear múltiplas mudanças num único fluxo com confirmação combinada (E2).
- Histórico/auditoria de longo prazo das edições e telas de UI (fora do canal conversacional).
- KPI de adoção de produto (não incluído no gate — R4).

## Suposições e Questões em Aberto

- Nenhuma questão em aberto. As decisões D-01..D-05 (US) e R1..R8, E1..E4 (rodadas de esclarecimento) foram confirmadas pelo usuário e estão registradas na tabela de Decisões Confirmadas.
- Suposição de desenho (não bloqueante, decidida na Especificação Técnica): a granularidade do workflow de edição (um workflow unificado `budget-edit` com slot de operação vs. definições irmãs por operação) cabe no kernel atual sem mudança de plataforma; ambas as formas atendem a todos os RFs acima.
