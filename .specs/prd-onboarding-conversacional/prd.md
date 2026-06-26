# Documento de Requisitos do Produto (PRD)

<!-- spec-version: 2 -->

> **Feature:** Onboarding conversacional do MeControla (WhatsApp), fiel ao Capítulo 08 do documento oficial, executado sobre o módulo `internal/agent` e o kernel `internal/platform/workflow`, com comunicação canônica entre módulos.
>
> **Fonte canônica de produto:** `docs/oficial/2026_06_24_mecontrola_oficial.md` (Cap. 07, 08, 09, 10, 11).
> **Fonte canônica de arquitetura/governança:** `AGENTS.md`, `.claude/rules/agent-workflows-tools.md`, `.claude/rules/workflow-kernel.md`.
> **Status:** discovery concluído; pronto para techspec.

---

## Visão Geral

O onboarding conversacional leva um usuário recém-ativado (que pagou e vinculou a assinatura via WhatsApp) de "não sei para onde meu dinheiro vai" para "tenho clareza sobre meu dinheiro e meus objetivos", criando o **primeiro planejamento financeiro** em poucos minutos, inteiramente por conversa no WhatsApp.

Hoje o repositório já possui uma base funcional: o `internal/agent` conduz o onboarding via um loop de fases dirigido por LLM (`run_onboarding_turn.go`), e o `internal/onboarding` é dono do estado durável (`onboarding_sessions`), dos use cases e dos eventos de domínio que propagam o resultado para `budgets` (`onboarding.splits_calculated`) e `card` (`onboarding.card_registered`). Contudo, a implementação atual **diverge do documento oficial em pontos materiais** e viola regras de modelagem do próprio repositório.

Este PRD define o produto-alvo do onboarding fiel ao Capítulo 08 (8 etapas), corrige as divergências confirmadas contra o código e estabelece, como **restrições e dependências** (não desenho técnico), o seam de comunicação entre módulos, os tipos de memória obrigatórios e o modelo de execução baseado no kernel de workflow durável. A transição para a Operação Diária (Cap. 09) é tratada apenas no ponto de fronteira (conclusão → primeira movimentação).

**Valor:** ativação completa do usuário pago, com planejamento consolidado e fiel à metodologia das 5 categorias oficiais, reduzindo abandono e criando a base de contexto (working memory) para a operação diária subsequente.

---

## Objetivos

- **O1 — Fidelidade ao oficial:** o fluxo cobre as 8 etapas do Capítulo 08 na ordem do Capítulo 07, com tom de voz, emojis e regras de comunicação dos Capítulos 03–06, sem flexibilizar etapas, transições ou objetivos.
- **O2 — Planejamento completo ao final:** ao concluir, o usuário possui objetivo definido, orçamento (renda) registrado, 0..N cartões cadastrados, distribuição nas 5 categorias oficiais e planejamento consolidado e apresentado.
- **O3 — Conformidade arquitetural:** o onboarding respeita `AGENTS.md` e as regras hard de `internal/agent` (R-AGENT-WF-001), do kernel (R-WF-KERNEL-001), DMMF (state-as-type, `Decide*` puro, smart constructors) e zero comentários em Go.
- **O4 — Robustez conversacional:** estado durável entre turnos (suspend/resume), retomada da etapa onde parou, correção de dados no resumo, e tratamento explícito de casos de borda (sem valor, sem meio de pagamento, ambiguidade, comando diário no meio do fluxo).
- **O5 — Observabilidade de funil:** cada execução é um `Run` auditável e cada etapa emite telemetria que permite medir conclusão e abandono por etapa.

### Métricas de sucesso (mensuráveis)

- **MS-01 — Taxa de conclusão:** % de sessões iniciadas (`onboarding.subscription_bound` → sessão `in_progress`) que atingem `onboarding.completed`. Meta inicial: ≥ 80%.
- **MS-02 — Tempo até conclusão:** mediana do tempo entre a primeira mensagem de onboarding e `onboarding.completed`. Meta: ≤ 10 min ("em poucos minutos", Cap. 08).
- **MS-03 — Abandono por etapa:** distribuição de `onboarding.step_abandoned` por etapa (funil), identificando a etapa de maior evasão.
- **MS-04 — Fidelidade de saída:** 100% das sessões concluídas possuem objetivo ≠ vazio, renda > 0 e exatamente 5 alocações de categoria (invariante de `IsReadyToComplete`).
- **MS-05 — Integridade de propagação:** 100% das sessões concluídas com cartão informado geram cartão em `internal/card`, e 100% com splits geram orçamento ativo em `internal/budgets` (idempotência por `event_id`, sem duplicidade).
- **MS-06 — Auditabilidade:** 100% das execuções de etapa possuem `Run` com `thread_id`, `run_id`, `status` (`RunStatus` fechado) e `duration_ms`.

---

## Histórias de Usuário

- **US-01 (Ator principal — usuário recém-ativado):** Como usuário que acabou de ativar minha assinatura pelo WhatsApp, quero ser recebido e guiado passo a passo, para criar meu primeiro planejamento financeiro sem precisar entender de finanças.
- **US-02 (Objetivo):** Como usuário, quero contar meu objetivo em linguagem natural ("quero quitar minhas dívidas"), para que o planejamento seja montado pensando nesse objetivo.
- **US-03 (Orçamento):** Como usuário, quero informar minha renda mensal de forma simples ("4000"), para definir o valor disponível do planejamento.
- **US-04 (Cartões):** Como usuário, quero cadastrar cartão informando só apelido e dia de vencimento (ou dizer que não uso cartão), para não precisar fornecer dados sensíveis.
- **US-05 (Categorias):** Como usuário leigo, quero entender a metodologia das 5 categorias antes de distribuir valores, para saber o que estou fazendo.
- **US-06 (Valores):** Como usuário, quero informar valores em reais por categoria e ver o percentual calculado automaticamente, para não ter que pensar em porcentagens.
- **US-07 (Resumo e correção):** Como usuário, quero revisar um resumo do meu planejamento e corrigir qualquer dado em linguagem natural antes de confirmar, para terminar com tudo certo.
- **US-08 (Conclusão e transição):** Como usuário, quero saber que meu planejamento está pronto e como registrar minhas movimentações, para começar a usar o produto no dia a dia.
- **US-09 (Retomada):** Como usuário que parou no meio, quero retomar de onde parei quando voltar a falar, para não recomeçar do zero.
- **US-10 (Desvio):** Como usuário ansioso, quero que, se eu mandar um gasto antes de terminar o setup, o MeControla me oriente gentilmente a concluir primeiro, sem perder minha mensagem de vista.
- **US-11 (Operação — produto):** Como time de produto, quero medir conclusão e abandono por etapa, para identificar e reduzir pontos de evasão do funil.

---

## Funcionalidades Core

### FC-01 — Fluxo guiado de 8 etapas (Cap. 08)
Sequência linear e durável: Boas-vindas → Objetivo → Orçamento (renda) → Cartões → Apresentação das Categorias → Valores das Categorias → Resumo Final → Conclusão. Uma pergunta por vez (Regra 1, Cap. 06); pergunta apenas o que falta (Regras 2–3); estado suspenso entre turnos e retomável.

### FC-02 — Condução conversacional por LLM no tom oficial
As mensagens são **geradas pelo LLM seguindo o tom de voz oficial** (Cap. 03–06), usando as "Mensagens Oficiais" do Cap. 08 como guia/exemplo (não literal). O LLM também interpreta a entrada do usuário, resolve ambiguidade e trata off-topic. O LLM atua exclusivamente no passo de parse/geração da resposta da etapa — nunca em regra de negócio, SQL ou branching de domínio.

### FC-03 — Apresentação e distribuição nas 5 categorias oficiais
Ensina a metodologia (💰 Custo Fixo, 🎓 Conhecimento, 🎉 Prazeres, 🎯 Metas, 🏦 Liberdade Financeira) e coleta **valores monetários** por categoria; o sistema calcula percentuais e exibe sempre "valor + percentual" (Cap. 10 — Regra de Distribuição). Nenhuma categoria adicional pode ser criada.

### FC-04 — Cadastro de 0..N cartões com privacidade
Solicita apenas apelido e dia de vencimento; nunca limite, banco, bandeira ou dados sensíveis (Cap. 10 — Regra de Cartões). Suporta múltiplos cartões ("Deseja adicionar outro?") e o caminho "não uso cartão".

### FC-05 — Resumo com correção guiada por LLM
Exibe o resumo consolidado e pergunta "Está tudo certo?". Se o usuário indicar correção em linguagem natural ("na verdade o orçamento é 5000"), o LLM identifica o campo e o novo valor, atualiza via use case e re-exibe o resumo; se ambíguo, pergunta qual campo ajustar.

### FC-06 — Conclusão e transição para Operação Diária
Ao confirmar o resumo, conclui o onboarding (emite `onboarding.completed`) **sem exigir primeira transação**; apresenta exemplos de uso diário (Cap. 08 — ETAPA 8). A primeira movimentação pertence à Operação Diária (Cap. 09).

### FC-07 — Memória de curto prazo e working memory
Histórico recente de turnos (recent_turns) disponível durante o fluxo; working memory consolidada na conclusão (perfil financeiro) para alimentar o system prompt da Operação Diária.

### FC-08 — Propagação de resultado entre módulos
Cartões e distribuição propagam-se aos módulos donos via domain events/outbox idempotentes já existentes (`onboarding.card_registered` → `card`; `onboarding.splits_calculated` → `budgets`).

### FC-09 — Telemetria de funil e abandono
Cada etapa é auditável como `Run` e emite sinal de progresso; inatividade prolongada em uma etapa gera `onboarding.step_abandoned` para métricas de funil.

---

## Requisitos Funcionais

### Entrada no onboarding e identidade
- **RF-01:** O onboarding inicia a partir de uma sessão `in_progress` criada na sequência de `onboarding.subscription_bound` (pós-ativação/vinculação de assinatura), conforme wiring atual; o PRD não altera o gatilho de entrada.
- **RF-02:** A identidade conversacional é o par `(user_id, channel=whatsapp)` (Thread), resolvido pelo runtime do agent antes de qualquer execução; toda execução abre/fecha um `Run` auditável.
- **RF-03:** Mensagens inbound são idempotentes por `messageID` (WAMID); reprocessamento não duplica efeitos nem avança etapa duas vezes.

### ETAPA 1 — Boas-vindas
- **RF-04:** A primeira interação apresenta o MeControla e termina com o convite "Vamos começar? 🚀", no tom oficial (LLM), e **aguarda a confirmação do usuário** ("Sim") antes de seguir para a ETAPA 2 (handshake oficial do Cap. 08 ETAPA 1). Registra que as boas-vindas foram enviadas (sem reenvio duplicado em replay/reentrada). A apresentação das 5 categorias **não** ocorre aqui (pertence à ETAPA 5).

### ETAPA 2 — Objetivo
- **RF-05:** O sistema solicita o objetivo do usuário e persiste o objetivo informado em linguagem natural; responde confirmando que o planejamento será montado para esse objetivo.

### ETAPA 3 — Orçamento (renda)
- **RF-06:** O sistema solicita o valor disponível do orçamento mensal e persiste a renda em centavos; confirma o valor registrado formatado em reais.
- **RF-07:** Entrada de renda sem valor reconhecível dispara pedido de esclarecimento no tom oficial, sem avançar a etapa (Cap. 11 — Receita Sem Valor).

### ETAPA 4 — Cartões
- **RF-08:** O sistema pergunta se o usuário usa cartão e solicita **apenas apelido e dia de vencimento da fatura** (due day), fiel ao Cap. 08 ETAPA 4 e ao Cap. 10. É proibido solicitar dia de fechamento, limite, banco, bandeira ou qualquer dado sensível. O onboarding **não** coleta dia de fechamento (`ClosingDay`); a reconciliação com o agregado `card` é tratada na techspec (QT-08).
- **RF-09:** O sistema suporta cadastrar múltiplos cartões em laço ("Deseja adicionar outro cartão?") e o caminho "não uso cartão" (zero cartões), avançando a etapa em ambos os casos.
- **RF-10:** Cada cartão informado confirma "Cartão salvo" com apelido e dia de vencimento e emite `onboarding.card_registered` (idempotente por `event_id`) para o módulo `card`.

### ETAPA 5 — Apresentação das Categorias
- **RF-11:** O sistema apresenta a metodologia das 5 categorias oficiais (💰 Custo Fixo, 🎓 Conhecimento, 🎉 Prazeres, 🎯 Metas, 🏦 Liberdade Financeira) e confirma o entendimento antes de coletar valores. Nenhuma categoria adicional pode ser criada pelo usuário.
- **RF-12:** Se o usuário não confirmar ou fizer pergunta sobre as categorias, o sistema esclarece brevemente no tom oficial e segue para a coleta de valores, sem travar o fluxo.

### ETAPA 6 — Valores das Categorias
- **RF-13:** O sistema coleta **valores monetários** para cada uma das 5 categorias, **uma por vez** (Regra 1, Cap. 06), confirmando cada valor registrado antes de perguntar o próximo. O usuário **sempre** informa os valores (Cap. 10 — Regra de Distribuição); é proibido auto-sugerir/pré-preencher a distribuição no caminho oficial do MVP.
- **RF-14:** O sistema calcula automaticamente os percentuais de cada categoria a partir dos valores informados; o usuário nunca informa percentuais.
- **RF-15:** Ao concluir a coleta, o resultado de distribuição é propagado ao módulo `budgets` via `onboarding.splits_calculated` (idempotente por `event_id`), criando e ativando o orçamento.

### ETAPA 7 — Resumo Final e correção
- **RF-16:** O sistema apresenta um resumo consolidado contendo objetivo, orçamento e a distribuição por categoria exibindo sempre **valor monetário + percentual**, e pergunta "Está tudo certo?".
- **RF-17:** Se o usuário indicar que algo não está certo, o sistema usa o LLM para identificar o campo e o novo valor a partir da fala em linguagem natural, atualiza o dado via use case e re-exibe o resumo; se a intenção for ambígua, pergunta qual campo ajustar.
- **RF-18:** A confirmação do resumo é um gate explícito de Human-in-the-Loop: o estado de espera é durável e tipado (estado fechado, não flag booleana/string livre) e o avanço só ocorre com confirmação do usuário.

### ETAPA 8 — Conclusão e transição
- **RF-19:** O onboarding conclui ao confirmar o resumo (ETAPA 7), **sem exigir registro de primeira transação**. O critério de prontidão exige objetivo ≠ vazio, renda > 0 e exatamente 5 alocações de categoria; a exigência de `FirstTxRecorded` é removida do critério de conclusão.
- **RF-20:** A conclusão emite `onboarding.completed` (idempotente por `event_id`) e apresenta a mensagem de conclusão com exemplos de uso da Operação Diária (ex.: "Mercado 120 pix", "Como estou esse mês?").
- **RF-21:** Após a conclusão, a working memory do usuário é consolidada (perfil financeiro) e disponibilizada no system prompt da Operação Diária; ausência de working memory não é erro.

### Estado, memória e retomada
- **RF-22:** O estado de progresso do onboarding é modelado como **tipo fechado de etapa** (state-as-type), substituindo a representação por string livre da fase atual; nunca string livre em assinatura pública.
- **RF-23:** O fluxo é durável e retomável: ao retornar, o usuário retoma a etapa em que parou (suspend/resume), preservando o estado já coletado; o estado suspenso é a fonte única de verdade (sem side-store paralelo de rascunho).
- **RF-24:** O histórico recente de turnos (recent_turns) é mantido e disponibilizado durante o fluxo para contexto curto; ao concluir, o histórico volátil de onboarding pode ser limpo conforme política do módulo.

### Casos de borda e comunicação
- **RF-25:** Durante o onboarding (antes da conclusão), comandos de Operação Diária (ex.: "Mercado 120 pix", "Como estou esse mês?") são reconhecidos e **adiados**: o sistema responde no tom oficial pedindo para concluir o setup primeiro, **sem registrar** a transação nem consultar; após a conclusão o comportamento diário é normal.
- **RF-26:** Entradas sem valor, sem meio de pagamento ou ambíguas em qualquer etapa disparam esclarecimento no tom oficial, sem avançar a etapa, seguindo as Regras de Comunicação (Cap. 06) e os casos do Cap. 11 aplicáveis ao onboarding.
- **RF-27:** A comunicação do onboarding com os módulos de domínio segue o seam canônico de `AGENTS.md`: **domain event/outbox** para efeitos que precisam sobreviver a crash/deploy (cartões → `card`, splits → `budgets`, conclusão → `agent`/`identity`) e **interface declarada pelo consumidor → use case** para leituras/validações síncronas (ex.: validação/resolução de categorias em `categories`, sugestão de alocação em `budgets`). Portas HTTP/handlers **não** são usadas como seam interno entre módulos.
- **RF-28:** Todo efeito propagado por evento é idempotente por `event_id`; entrega é at-least-once e o consumidor não duplica cartão, orçamento nem conclusão em reprocessamento.

### Auditoria e funil
- **RF-29:** Cada execução de etapa é observável como `Run` auditável contendo no mínimo `thread_id`, `run_id`, `status` (`RunStatus` fechado), `duration_ms` e `error` quando houver.
- **RF-30:** O sistema emite `onboarding.step_abandoned` (ou sinal equivalente) quando uma sessão fica inativa numa etapa além de um limite definido, e expõe métricas de funil por etapa com **cardinalidade controlada** (sem `user_id`/`correlation_key`/`category_id` como label).

---

## Experiência do Usuário

**Persona primária:** usuário leigo em finanças, recém-pago, interagindo só por texto no WhatsApp. Espera simplicidade, linguagem natural e uma pergunta por vez.

**Fluxo principal (feliz):**
1. Boas-vindas → "Vamos começar?" → usuário aceita.
2. Objetivo em linguagem natural → confirmação ("Vamos montar tudo pensando nisso").
3. Renda mensal → "Orçamento registrado — R$ 4.000".
4. Cartões (apelido + vencimento, laço de N cartões ou "não uso").
5. Apresentação das 5 categorias → "Faz sentido?".
6. Valores por categoria, uma a uma, com confirmação a cada uma.
7. Resumo (valor + percentual por categoria) → "Está tudo certo?" → correção em linguagem natural se necessário.
8. Conclusão → "Seu planejamento está pronto!" + exemplos de uso diário.

**Casos de borda relevantes (UX):**
- Correção no resumo via fala natural (RF-17).
- Comando diário no meio do fluxo → redirecionamento gentil (RF-25).
- Retomada da etapa onde parou ao voltar (RF-23).
- Entrada sem valor/ambígua → re-pergunta no tom oficial (RF-26).
- "Não uso cartão" → pula cadastro sem fricção (RF-09).

**Diretrizes obrigatórias de comunicação (Cap. 03–06):** tom acolhedor e direto; uma pergunta por vez; não pedir o que já foi informado; priorizar ação; emojis oficiais (Cap. 05); clareza visual com blocos curtos.

> **Requisito de validação de UX (governança do projeto):** dada a exigência histórica de fidelidade de runbook/diálogo, a techspec/implementação deve acompanhar um runbook de jornada completa do onboarding com exemplos de diálogo verbatim por etapa, 1:1 ao código, cobrindo os casos de borda acima.

---

## Restrições Técnicas de Alto Nível

> Capturadas como **restrições, dependências e premissas já validadas no código**. Desenho detalhado pertence à Especificação Técnica.

### Restrições de arquitetura e governança (não negociáveis)
- **RT-01 — Roteamento canônico do agent (R-AGENT-WF-001):** novo comportamento entra como `Workflow`/`Tool` reutilizando bindings/usecases; **proibido** adicionar `case intent.Kind` de domínio ao switch de `daily_ledger_agent.go`. Tool é adapter fino (sem regra de negócio, SQL ou branching). `ToolOutcome`/`RunStatus`/`AwaitingKind`/estado de etapa são tipos fechados.
- **RT-02 — Modelo de execução = kernel de workflow durável:** o fluxo de 8 etapas é executado sobre `internal/platform/workflow` (`Engine[S]`, steps, suspend/resume, `Codec.MergePatch` RFC 7386), com `Run`/etapa auditável. O agent é o consumidor do kernel; a semântica de Thread/WorkingMemory/PendingStep permanece exclusiva de `internal/agent`.
- **RT-03 — Kernel genérico (R-WF-KERNEL-001):** `internal/platform/workflow` não pode importar pacote de domínio, conter regra/branching de domínio, LLM ou SQL fora do adapter Postgres; estados (`RunStatus`/`StepStatus`/`SuspendReason`) são tipos fechados. O estado de onboarding entra como `S` genérico/opaco.
- **RT-04 — LLM apenas no parse/geração (R-AGENT-WF-001.4):** LLM é usado para interpretar entrada e gerar a resposta no tom oficial; proibido LLM em regra de negócio, no gate de confirmação do resumo ou em qualquer step de domínio.
- **RT-05 — DMMF:** state-as-type para etapa e estados de espera; `Decide*` puro (sem IO/`context.Context`) para qualquer regra (ex.: validação de prontidão, cálculo de percentuais se feito no domínio do onboarding); smart constructors em VOs/commands; pipeline `parse → validate → decide → persist → publish`. Anti-padrões proibidos: `Result[T,E]` custom, currying, DSL de pipeline, mon,Either.
- **RT-06 — Zero comentários em Go de produção** (R-ADAPTER-001.1) e adaptadores finos `adapter → usecase` nos quatro caminhos de adapter.
- **RT-07 — Seam de comunicação cross-module:** domain event/outbox para efeitos duráveis; interface no consumidor → use case para leituras/validações síncronas; **HTTP handler não é seam interno**. Idempotência por `event_id` obrigatória.
- **RT-08 — Cardinalidade de métricas controlada:** sem `user_id`/`correlation_key`/`category_id` como label (R-TXN-004 / R-WF-KERNEL-001.4).

### Dependências de módulo (estado atual confirmado no código)
- **RT-09:** `internal/onboarding` é dono do estado durável (`onboarding_sessions`, JSONB) e dos use cases de objetivo/renda/cartão/splits/conclusão/fase/turns/contexto; emite `onboarding.card_registered`, `onboarding.splits_calculated`, `onboarding.completed`, `onboarding.subscription_bound`.
- **RT-10:** `internal/budgets` já consome `onboarding.splits_calculated` (CreateBudget + ActivateBudget) e expõe `SuggestAllocation` (consumido via binding pelo onboarding).
- **RT-11:** `internal/card` já consome `onboarding.card_registered` (CreateCard).
- **RT-12:** `internal/categories` é read-only (sem criação pelo usuário); expõe `ResolveBySlug`/`ValidateSubcategory`/`VersionReader` via interface no consumidor.
- **RT-13:** `internal/agent` já possui Thread, Run, WorkingMemory, Observation, dispatcher de onboarding, phase setter e history gateway; o onboarding consome o kernel via o runtime do agent.
- **RT-14:** Canal `internal/platform/whatsapp`: dispatcher stateless, texto apenas (~4096 chars/mensagem), identidade `+E.164`, idempotência por WAMID, janela de timestamp de 5 min; o estado conversacional vive no onboarding/kernel, não no canal.

### Memória (decisão de produto)
- **RT-15:** Obrigatórias no MVP — **message history (recent_turns)** durante o fluxo e **working memory** consolidada na conclusão. **Observation memory** é útil-opcional. **Semantic recall** (embeddings/busca vetorial) está **fora do MVP**.

### Compliance e privacidade
- **RT-16:** Proibido coletar/armazenar dados sensíveis de cartão (limite, banco, bandeira, número) — apenas apelido e dia de vencimento (Cap. 10).
- **RT-17:** Conteúdo de prompt/decisão LLM deve seguir o padrão de auditoria/redação já existente no agent (hash de prompt, resposta redigida) quando aplicável.

### Performance/operação
- **RT-18:** Latência de resposta por turno compatível com a janela de 5 min do webhook e a experiência "em poucos minutos"; sem bloquear o webhook (processamento assíncrono via outbox, como já ocorre).

---

## Fora de Escopo

- **FE-01:** Alterar o gatilho de entrada no onboarding (ativação/magic token/`subscription_bound`) — permanece como está.
- **FE-02:** Operação Diária completa (Cap. 09): registro de receitas/despesas, cartão, parcelamento, consultas, alteração/exclusão — apenas a **fronteira** de transição é coberta (conclusão + redirecionamento de comandos diários durante o onboarding).
- **FE-03:** Semantic recall / memória de longo prazo por embeddings.
- **FE-04:** Criação de categorias customizadas pelo usuário (proibido por regra oficial).
- **FE-05:** Canais além do WhatsApp (Telegram foi eliminado; sem multicanal).
- **FE-06:** Coleta de limite/banco/bandeira/dados sensíveis de cartão.
- **FE-07:** Reset/recomeço total como fluxo de produto novo — o reset existente em `StartBudgetConfiguration` é mantido, mas não é redesenhado aqui.
- **FE-08:** Onboarding orientado por menus/botões interativos do WhatsApp (a interação é por texto livre interpretado por LLM).
- **FE-09:** Detalhamento de implementação (estrutura de steps, nomes de tipos, DDL, contratos de função) — pertence à Especificação Técnica.

---

## Suposições e Questões em Aberto

### Suposições assumidas (confirmadas no código durante o discovery)
- **SP-01:** A propagação cartões→`card` e splits→`budgets` por domain event já existe e é idempotente; o PRD a mantém como seam canônico (não recriar via HTTP).
- **SP-02:** O kernel `internal/platform/workflow` suporta o fluxo linear de 8 etapas com suspend/resume durável e merge-patch de estado (validado: `Engine.Start`/`Resume`, cursor, `Codec.MergePatch`).
- **SP-03:** O `internal/onboarding` permanece dono do estado durável e dos use cases; o `internal/agent` conduz o turno e consome o kernel — arranjo híbrido alinhado a `AGENTS.md` (Thread/Run/WorkingMemory exclusivos do agent).
- **SP-04:** As mensagens são geradas por LLM no tom oficial; as "Mensagens Oficiais" do Cap. 08 são guia de conteúdo/tom, não strings literais obrigatórias.

### Decisões registradas neste discovery (já fechadas)
- **D-01 (mensagens):** LLM no tom oficial (não verbatim). → FC-02, RF-04/RF-16.
- **D-02 (conclusão):** Alinhar ao oficial — concluir na ETAPA 8 sem exigir primeira transação; remover `FirstTxRecorded` do critério. → RF-19.
- **D-03 (execução):** Migrar o fluxo para o kernel `internal/platform/workflow` (suspend/resume, Run auditável por etapa, etapa como tipo fechado). → RT-02, RF-22/RF-23/RF-29.
- **D-04 (memória):** History + Working obrigatórias; Observation opcional; Semantic recall fora do MVP. → RT-15.
- **D-05 (correção no resumo):** Correção guiada por LLM. → RF-17.
- **D-06 (comando diário no fluxo):** Redirecionar gentilmente sem registrar. → RF-25.
- **D-07 (abandono):** Incluir rastreio de abandono no MVP (`onboarding.step_abandoned` + funil por etapa). → RF-30.
- **D-08 (cartão — só vencimento):** Seguir o oficial — o onboarding coleta **apenas apelido + dia de vencimento da fatura**; não coleta dia de fechamento, limite, banco ou bandeira. → RF-08/RF-10, QT-08.
- **D-09 (handshake de boas-vindas):** Incluir o turno "Vamos começar? → Sim" como passo próprio (Cap. 08 ETAPA 1), sem pedir objetivo nem apresentar categorias no welcome. → RF-04.
- **D-10 (sem auto-sugestão de split):** Na ETAPA 6 o usuário sempre informa os valores, categoria por categoria; `suggest_budget_split`/auto-preview saem do caminho oficial do MVP. → RF-13.

### Lacunas confirmadas entre o documento oficial e o código atual (insumo para a techspec)
- **L-01:** ETAPA 5 (Apresentação das Categorias) não tem suporte explícito hoje — precisa ser adicionada (RF-11/RF-12).
- **L-02:** ETAPA 7 (Resumo + confirmação) não tem use case/gate hoje — precisa de gate HITL durável + correção (RF-16/RF-17/RF-18).
- **L-03:** A fase do onboarding é string livre — viola DMMF state-as-type; deve virar tipo fechado (RF-22).
- **L-04:** `IsReadyToComplete()` exige `FirstTxRecorded`, divergindo do Cap. 07 — ajustar (RF-19).
- **L-05:** As fases atuais do agent (`welcome/objective/budget/cards/financial_plan/first_tx`) não mapeiam 1:1 às 8 etapas oficiais — remodelar a sequência para as 8 etapas.
- **L-06:** Não há rastreio de abandono/funil — adicionar (RF-30).

### Questões deixadas explicitamente para a Especificação Técnica
- **QT-01:** Desenho dos steps do kernel para as 8 etapas (sequência, granularidade, branch do laço de cartões e do caminho "não uso cartão") e como o `OnboardingState` é representado como `S` opaco.
- **QT-02:** Estratégia de gate HITL do Resumo reusando os primitivos de confirmação/suspend existentes (sem expor tipo de domínio ao kernel) e contrato de resume via merge-patch.
- **QT-03:** Definição do tipo fechado de etapa (`OnboardingPhase`/estado) e migração do campo `phase` (string) no payload/`onboarding_sessions` sem quebrar sessões em andamento.
- **QT-04:** Limite de inatividade e mecanismo de emissão de `onboarding.step_abandoned` (job/housekeeping do kernel vs. avaliação no resume) e labels de métrica de funil com cardinalidade controlada.
- **QT-05:** Onde reside o cálculo de percentuais (domínio do onboarding via `Decide*` puro vs. já coberto por `budgets` na ativação) e como evitar duplicação de regra.
- **QT-06:** Mapa `OperationKind`/estado fechado para o redirecionamento de comandos diários durante o onboarding (reconhecer intent diário sem executar) sem crescer switch de domínio.
- **QT-07:** Política de limpeza de `recent_turns` na conclusão e formato/conteúdo exato da working memory consolidada.
- **QT-08 (reconciliação do cartão):** Como criar o cartão em `internal/card` coletando **apenas o dia de vencimento** (decisão D-08), dado que `card.CreateCard` hoje exige `ClosingDay` (1–31) e tem `DueDay` opcional. Opções a fechar na techspec: tornar `DueDay` o dado obrigatório do onboarding e derivar/assumir `ClosingDay`, ou ajustar o contrato do módulo `card` — sem pedir o fechamento ao usuário.

---

## Apêndice — Recomendação preliminar de arquitetura (insumo, não desenho final)

> Separação `recomendado` / `aceitável com trade-off` / `evitar`, conforme solicitado no discovery. Decisão final de desenho é da techspec.

### Comunicação entre módulos
- **Recomendado:** domain event/outbox idempotente para efeitos duráveis (cartões→`card`, splits→`budgets`, conclusão→`agent`/`identity`); interface no consumidor → use case para leituras/validações síncronas (categorias, sugestão de alocação). É o padrão já existente e o seam canônico de `AGENTS.md`.
- **Aceitável com trade-off:** binding síncrono direto (interface no consumidor → use case) para criar cartão/splits caso se exija feedback imediato e transacional — custo: acoplamento mais forte e perda da resiliência at-least-once do outbox.
- **Evitar:** chamar portas HTTP/handlers de outro módulo como seam interno; compartilhar transação/repositório entre módulos.

### Memória
- **Recomendado:** message history (recent_turns) + working memory consolidada na conclusão.
- **Aceitável com trade-off:** observation memory durante o fluxo (notas curtas com TTL) — útil para personalização, custo de complexidade/armazenamento.
- **Evitar:** semantic recall/embeddings no MVP (custo e infraestrutura inexistente hoje).

### Workflow / Tool / Agente
- **Recomendado:** híbrido — agent conduz o turno (LLM no parse/geração) e o fluxo de 8 etapas roda sobre o kernel de workflow durável (suspend/resume, Run auditável), com etapa como tipo fechado e gate HITL no resumo reusando primitivos existentes.
- **Aceitável com trade-off:** manter parte da orquestração no use case `run_onboarding_turn` enquanto migra incrementalmente etapas para steps do kernel — custo: dualidade temporária de modelo de execução.
- **Evitar:** criar novo `case intent.Kind` de domínio no switch do agent; colocar regra de negócio/LLM dentro do kernel; representar etapa/estados de espera como string livre.

### Necessidade de Tool nova
- **Recomendado:** reutilizar as tools/dispatchers de onboarding já existentes (objetivo, renda, cartão, splits, conclusão); criar Tool nova **apenas** para ETAPA 5 (apresentação) e para o gate/correção do Resumo (ETAPA 7), se um handler binding/usecase específico for necessário.
- **Evitar:** Tool que contenha regra de negócio, SQL ou branching de domínio (deve ser adapter fino).
